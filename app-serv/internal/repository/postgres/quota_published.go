// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota_published.go
// @for       The published-quota cache read path: the screen's one batched
//
//	query over the cached answers, and the sweep's due-endpoint queue.
//
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgxpool,
//
//	internal/domain, internal/repository, context, time.
//
// @reason    The quota screen shows what each provider publishes about itself,
//
//	and it may not fetch that on read: a page of accounts would become a page
//	of provider calls, the exact N+1 AGENTS.md §1.7 blocks on this screen. A
//	worker writes the answers (quota_published_write.go) and this file reads
//	them back in ONE statement for the whole batch. The two halves are split
//	because they change for different reasons — a new display column here, a
//	new scheduling rule there — and §1.1 caps the file either way.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-02
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// publishedStateColumns is the state projection both reads use, in scan order.
const publishedStateColumns = `s.endpoint_id, s.provider_id, s.plan, s.message,
	s.fetched_at, s.last_attempt_at, s.next_attempt_at, s.consecutive_failures`

// publishedWindowColumns is the window half of the batched read. used and total
// come back as text: a numeric column decoded into a float would round the same
// way SPEC-API-001 §4 forbids on the wire, and the domain type is a decimal
// string. A NULL total stays a NULL, which is the difference between an unlimited
// allowance and a spent one (migrations/000013).
const publishedWindowColumns = `w.label, w.used::text, w.total::text,
	w.unlimited, w.is_credit_balance, w.recurring, w.unit, w.resets_at, w.fetched_at`

// PublishedQuotaRepository persists the provider-published quota cache.
type PublishedQuotaRepository struct {
	pool *pgxpool.Pool
}

// NewPublishedQuotaRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewPublishedQuotaRepository(pool *pgxpool.Pool) *PublishedQuotaRepository {
	return &PublishedQuotaRepository{pool: pool}
}

// ListPublishedByEndpointIDs returns every cached answer for the batch, keyed by
// endpoint id, in ONE statement.
//
// The batch is the point: the screen's read path passes the ids of the page it is
// rendering and must not pay a round trip per account (§1.7), so the filter is an
// array membership test rather than a built-up IN list. The join is from state to
// window, not the reverse, because an endpoint whose provider answered with no
// buckets still has a plan and a fetched_at to show; the invariant that a window
// row never stands alone is held by StorePublished, which writes both in one
// transaction.
//
// An empty id list costs no query at all — a scan over an empty array is still a
// round trip, and a page with no rows should cost nothing.
func (r *PublishedQuotaRepository) ListPublishedByEndpointIDs(ctx context.Context, endpointIDs []string) (map[string]domain.PublishedQuota, error) {
	out := make(map[string]domain.PublishedQuota, len(endpointIDs))
	if len(endpointIDs) == 0 {
		return out, nil
	}
	const q = `SELECT ` + publishedStateColumns + `, ` + publishedWindowColumns + `
	  FROM quota_published_state s
	  LEFT JOIN quota_published_window w ON w.endpoint_id = s.endpoint_id
	 WHERE s.endpoint_id = ANY($1::text[])
	 ORDER BY s.endpoint_id ASC, w.label ASC`

	rows, err := r.pool.Query(ctx, q, endpointIDs)
	if err != nil {
		return nil, translatePublishedQuotaError(err)
	}
	defer rows.Close()

	for rows.Next() {
		state, label, window, err := scanPublishedRow(rows)
		if err != nil {
			return nil, err
		}
		quota, seen := out[state.EndpointID]
		if !seen {
			quota = domain.PublishedQuota{
				State:   state,
				Windows: make([]domain.PublishedWindowRow, 0, 4),
			}
		}
		if label != "" {
			quota.Windows = append(quota.Windows, window)
		}
		out[state.EndpointID] = quota
	}
	if err := rows.Err(); err != nil {
		return nil, translatePublishedQuotaError(err)
	}
	return out, nil
}

// DueForRefresh returns the endpoints whose next attempt has come due, oldest
// first, at most limit of them.
//
// The order is the sweep's fairness guarantee: the endpoint waiting longest is
// polled first, so a limit smaller than the due set rotates coverage across the
// fleet instead of refreshing the same heads forever. The tie-break on id keeps a
// tick reproducible. `next_attempt_at <= $1` is served by the index 000013
// declares, which makes the sweep a range scan with a bounded read rather than a
// sort of the whole table.
func (r *PublishedQuotaRepository) DueForRefresh(ctx context.Context, now time.Time, limit int) ([]domain.PublishedState, error) {
	if limit < 1 {
		return nil, nil
	}
	// The cooldown filter sits in the WHERE, before the LIMIT, not after the rows come back:
	// a throttled endpoint that still occupies an oldest-due slot would eat budget every tick
	// and starve the accounts behind it. `rate_limited_until` is the deadline the data plane
	// already sets when that endpoint answers 429 to real traffic, so the poller respects the
	// cooldown an operator's requests have already proven necessary (AGENTS.md §1.6).
	const q = `SELECT ` + publishedStateColumns + `
	  FROM quota_published_state s
	 WHERE s.next_attempt_at <= $1
	   AND NOT EXISTS (
	        SELECT 1 FROM upstream_endpoints e
	         WHERE e.id = s.endpoint_id AND e.rate_limited_until > $1)
	 ORDER BY s.next_attempt_at ASC, s.endpoint_id ASC
	 LIMIT $2`

	rows, err := r.pool.Query(ctx, q, now, limit)
	if err != nil {
		return nil, translatePublishedQuotaError(err)
	}
	defer rows.Close()

	due := make([]domain.PublishedState, 0, limit)
	for rows.Next() {
		state, err := scanPublishedState(rows)
		if err != nil {
			return nil, err
		}
		due = append(due, state)
	}
	if err := rows.Err(); err != nil {
		return nil, translatePublishedQuotaError(err)
	}
	return due, nil
}
