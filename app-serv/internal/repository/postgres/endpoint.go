// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint.go
// @for       PostgreSQL persistence for the UpstreamEndpoint aggregate root.
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository,
//
//	context.
//
// @reason    SPEC-API-001 §6 defines upstream_endpoints and §7.5 makes the
//
//	endpoint the mutation boundary for its keys, so AGENTS.md §2.2
//	requires this repository to save the root — endpoint and keys
//	together — and load it the same way. Keys for a whole page are
//	fetched in one statement rather than per row (§1.7). A key's own
//	statements live in endpoint_keys.go, its row codec in endpoint_row.go,
//	and the driver-error mapping in endpoint_errors.go; this file owns the
//	root's statements.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// EndpointRepository persists upstream endpoints and their keys in PostgreSQL.
type EndpointRepository struct {
	pool *pgxpool.Pool
}

// NewEndpointRepository binds the repository to a pool whose limits the
// composition root sets explicitly (AGENTS.md §1.7).
func NewEndpointRepository(pool *pgxpool.Pool) *EndpointRepository {
	return &EndpointRepository{pool: pool}
}

// Create persists a new endpoint with its keys in one transaction, so a rejected
// key cannot leave an account that exists but cannot authenticate (AGENTS.md
// §2.2). A duplicate (provider_id, label) maps to domain.ErrEndpointExists for
// the service to return 409.
func (r *EndpointRepository) Create(ctx context.Context, endpoint domain.UpstreamEndpoint) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		return insertEndpoint(ctx, tx, endpoint)
	})
}

// List returns one page of endpoints with their keys attached, narrowed by
// provider and status. The order is provider, then priority, then id: it is the
// order the router reads, so it is the order the panel must show.
//
// The total comes from a window function in the same statement, so the count and
// the page always describe one snapshot. Keys for the whole page are read in one
// statement keyed by the page's ids, so a page of 25 endpoints costs two queries
// rather than 26 (AGENTS.md §1.7).
func (r *EndpointRepository) List(ctx context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	const query = `
SELECT ` + endpointColumns + `, count(*) OVER() AS total
  FROM upstream_endpoints
 WHERE ($1 = '' OR provider_id = $1)
   AND ($2 = '' OR status = $2)
 ORDER BY provider_id, priority, id
 LIMIT $3 OFFSET $4`
	const countQuery = `
SELECT count(*) FROM upstream_endpoints
 WHERE ($1 = '' OR provider_id = $1)
   AND ($2 = '' OR status = $2)`

	rows, err := r.pool.Query(ctx, query, filter.ProviderID, filter.Status, q.PerPage, q.Offset())
	if err != nil {
		return nil, 0, translateEndpointError(err)
	}
	defer rows.Close()

	rowsByID := make(map[string]endpointRow, q.PerPage)
	orderedIDs := make([]string, 0, q.PerPage)
	var total int64
	for rows.Next() {
		var rowTotal int64
		row, err := scanEndpointRow(rows, &rowTotal)
		if err != nil {
			return nil, 0, translateEndpointError(err)
		}
		total = rowTotal
		rowsByID[row.id] = row
		orderedIDs = append(orderedIDs, row.id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateEndpointError(err)
	}

	// A page beyond the last row returns no rows and therefore no window count;
	// the total is then read separately so pagination still reports it.
	if len(orderedIDs) == 0 {
		if err := r.pool.QueryRow(ctx, countQuery, filter.ProviderID, filter.Status).Scan(&total); err != nil {
			return nil, 0, translateEndpointError(err)
		}
		return make([]domain.UpstreamEndpoint, 0), total, nil
	}

	keysByEndpoint, err := r.keysForEndpoints(ctx, orderedIDs)
	if err != nil {
		return nil, 0, err
	}

	// The page is reassembled in the order the query returned it: reading the ids
	// back out of a map would discard the provider/priority ordering and hand the
	// panel a list the router does not agree with.
	page := make([]domain.UpstreamEndpoint, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		endpoint, err := rowsByID[id].aggregate(keysByEndpoint[id])
		if err != nil {
			return nil, 0, err
		}
		page = append(page, endpoint)
	}
	return page, total, nil
}

// GetByID loads one endpoint with its keys. A missing row yields
// domain.ErrEndpointNotFound.
func (r *EndpointRepository) GetByID(ctx context.Context, id string) (domain.UpstreamEndpoint, error) {
	const q = `SELECT ` + endpointColumns + ` FROM upstream_endpoints WHERE id = $1`

	row, err := scanEndpointRow(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return domain.UpstreamEndpoint{}, translateEndpointError(err)
	}
	keysByEndpoint, err := r.keysForEndpoints(ctx, []string{id})
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	return row.aggregate(keysByEndpoint[id])
}

// Update persists the endpoint's own fields, including its OAuth state (stored
// as ciphertext) and its account identity. Keys are untouched: they have their
// own methods because a key change is a different concern.
func (r *EndpointRepository) Update(ctx context.Context, endpoint domain.UpstreamEndpoint) error {
	oauthJSON, err := marshalOAuth(endpoint.OAuth())
	if err != nil {
		return err
	}
	accountJSON, err := marshalAccount(endpoint.Account())
	if err != nil {
		return err
	}
	testJSON, err := marshalTestStatus(endpoint.TestStatus())
	if err != nil {
		return err
	}
	const q = `
UPDATE upstream_endpoints
   SET label = $1,
       priority = $2,
       status = $3,
       oauth = $4,
       account = $5,
       test_status = $6,
       rate_limited_until = $7,
       last_used_at = $8,
       updated_at = $9
 WHERE id = $10`

	tag, err := r.pool.Exec(ctx, q, endpoint.Label(), endpoint.Priority(),
		string(endpoint.Status()), oauthJSON, accountJSON, testJSON,
		endpoint.RateLimitedUntil(), endpoint.LastUsedAt(),
		endpoint.UpdatedAt(), endpoint.ID())
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}

// Delete removes the endpoint; its keys go with it through the ON DELETE
// CASCADE the migration declares.
func (r *EndpointRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM upstream_endpoints WHERE id = $1`, id)
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}

// CountEndpoints reports how many endpoints reference a provider id, which is
// what lets a node delete refuse while one still does (domain.ErrNodeInUse).
func (r *EndpointRepository) CountEndpoints(ctx context.Context, providerID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM upstream_endpoints WHERE provider_id = $1`, providerID).Scan(&count)
	if err != nil {
		return 0, translateEndpointError(err)
	}
	return count, nil
}

// keysForEndpoints loads the keys of several endpoints in one statement, keyed
// by endpoint id. The id list comes from rows this repository read a moment ago,
// so it is bounded by the page size and the IN list cannot grow without limit
// (AGENTS.md §1.7).
func (r *EndpointRepository) keysForEndpoints(ctx context.Context, endpointIDs []string) (map[string][]domain.UpstreamKey, error) {
	out := make(map[string][]domain.UpstreamKey, len(endpointIDs))
	if len(endpointIDs) == 0 {
		return out, nil
	}
	const q = `
SELECT ` + upstreamKeyColumns + `
  FROM upstream_keys
 WHERE endpoint_id = ANY($1)
 ORDER BY endpoint_id, priority, id`

	rows, err := r.pool.Query(ctx, q, endpointIDs)
	if err != nil {
		return nil, translateKeyError(err)
	}
	defer rows.Close()

	for rows.Next() {
		key, err := scanUpstreamKey(rows)
		if err != nil {
			return nil, translateKeyError(err)
		}
		out[key.EndpointID()] = append(out[key.EndpointID()], key)
	}
	if err := rows.Err(); err != nil {
		return nil, translateKeyError(err)
	}
	return out, nil
}
