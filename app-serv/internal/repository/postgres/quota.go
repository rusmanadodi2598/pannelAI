// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/quota.go
// @for       PostgreSQL persistence for quota windows and endpoint budget caps.
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    SPEC-API-001 §7.12 reads windows per endpoint and per collection,
//
//	and §6 stores a cap as one row per endpoint. The flush worker writes
//	a whole batch, so the upsert is set-based: a statement per window
//	would make a flush of a thousand endpoints a thousand round trips
//	(AGENTS.md §1.7).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// quotaWindowColumns is the projection every window read uses, in scan order.
// The provider id is joined, not stored: it is a property of the endpoint, and
// copying it into a counter row would let the two disagree after a move.
const quotaWindowColumns = `w.endpoint_id, coalesce(e.provider_id, ''), w.window,
	w.used_units, w.limit_units, w.resets_at, w.source, w.updated_at`

// QuotaRepository persists quota windows and budget caps.
type QuotaRepository struct {
	pool *pgxpool.Pool
}

// NewQuotaRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewQuotaRepository(pool *pgxpool.Pool) *QuotaRepository {
	return &QuotaRepository{pool: pool}
}

// ListWindows returns windows ordered by endpoint then kind, so the collection
// route's ordering is stable and a client diffing two responses sees a real
// change rather than a reordering. An empty endpointID lists every endpoint.
func (r *QuotaRepository) ListWindows(ctx context.Context, endpointID string) ([]domain.QuotaWindow, error) {
	// The provider id is joined rather than stored on the window: it is a
	// property of the endpoint, and copying it into a counter row would let the
	// two disagree after an endpoint is moved between providers.
	const q = `SELECT ` + quotaWindowColumns + `
	  FROM quota_windows w
	  LEFT JOIN upstream_endpoints e ON e.id = w.endpoint_id
	 WHERE ($1 = '' OR w.endpoint_id = $1)
	 ORDER BY w.endpoint_id ASC, w.window ASC`

	rows, err := r.pool.Query(ctx, q, endpointID)
	if err != nil {
		return nil, translateQuotaError(err)
	}
	defer rows.Close()

	windows := make([]domain.QuotaWindow, 0, 8)
	for rows.Next() {
		window, err := scanQuotaWindow(rows)
		if err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	if err := rows.Err(); err != nil {
		return nil, translateQuotaError(err)
	}
	return windows, nil
}

// UpsertWindows persists a flushed batch in one statement.
//
// The row is the durable record, and the flush overwrites `used_units` rather
// than adding to it: the Redis counter is authoritative until it is cleared, so
// an incremental update here would double-count a batch that is retried after a
// partial failure. `resets_at` and `source` are carried through unchanged except
// where the caller supplied them.
func (r *QuotaRepository) UpsertWindows(ctx context.Context, windows []domain.QuotaWindow) error {
	if len(windows) == 0 {
		return nil
	}
	const q = `
INSERT INTO quota_windows
    (endpoint_id, window, used_units, limit_units, resets_at, source, updated_at)
SELECT * FROM unnest($1::text[], $2::text[], $3::bigint[], $4::bigint[],
                     $5::timestamptz[], $6::text[])
ON CONFLICT (endpoint_id, window) DO UPDATE
   SET used_units = EXCLUDED.used_units,
       limit_units = EXCLUDED.limit_units,
       resets_at = EXCLUDED.resets_at,
       source = EXCLUDED.source,
       updated_at = now()`

	// A nil limit must encode as SQL NULL rather than 0, because "uncapped" and
	// "capped at zero" are different rules for the router.
	endpoints := make([]string, 0, len(windows))
	kinds := make([]string, 0, len(windows))
	used := make([]int64, 0, len(windows))
	limits := make([]*int64, 0, len(windows))
	resets := make([]*time.Time, 0, len(windows))
	sources := make([]string, 0, len(windows))
	for _, window := range windows {
		endpoints = append(endpoints, window.EndpointID())
		kinds = append(kinds, string(window.Window()))
		used = append(used, window.Used())
		if limit, ok := window.Limit(); ok {
			value := limit
			limits = append(limits, &value)
		} else {
			limits = append(limits, nil)
		}
		resets = append(resets, window.ResetsAt())
		sources = append(sources, string(window.Source()))
	}

	if _, err := r.pool.Exec(ctx, q, endpoints, kinds, used, limits, resets, sources); err != nil {
		return translateQuotaError(err)
	}
	return nil
}

// GetCap loads one endpoint's budget cap. A missing row yields
// domain.ErrQuotaCapNotFound.
func (r *QuotaRepository) GetCap(ctx context.Context, endpointID string) (domain.QuotaCap, error) {
	const q = `SELECT endpoint_id, monthly_cost_usd::text, monthly_tokens, updated_at
	  FROM quota_caps WHERE endpoint_id = $1`

	var (
		id        string
		costText  *string
		tokens    *int64
		updatedAt time.Time
	)
	if err := r.pool.QueryRow(ctx, q, endpointID).Scan(&id, &costText, &tokens, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.QuotaCap{}, domain.ErrQuotaCapNotFound
		}
		return domain.QuotaCap{}, translateQuotaError(err)
	}

	var cost *domain.Decimal
	if costText != nil {
		parsed, err := domain.ParseDecimal(*costText)
		if err != nil {
			return domain.QuotaCap{}, err
		}
		cost = &parsed
	}
	return domain.RehydrateQuotaCap(id, cost, tokens, updatedAt), nil
}

// SetCap replaces one endpoint's cap. A nil field stores SQL NULL, which the
// read side reports as "no cap" rather than a cap of zero.
func (r *QuotaRepository) SetCap(ctx context.Context, cap domain.QuotaCap) error {
	const q = `
INSERT INTO quota_caps (endpoint_id, monthly_cost_usd, monthly_tokens, updated_at)
VALUES ($1, $2::numeric, $3, now())
ON CONFLICT (endpoint_id) DO UPDATE
   SET monthly_cost_usd = EXCLUDED.monthly_cost_usd,
       monthly_tokens = EXCLUDED.monthly_tokens,
       updated_at = now()`

	var cost *string
	if text, ok := cap.MonthlyCostUSD(); ok {
		cost = &text
	}
	var tokens *int64
	if value, ok := cap.MonthlyTokens(); ok {
		tokens = &value
	}

	if _, err := r.pool.Exec(ctx, q, cap.EndpointID(), cost, tokens); err != nil {
		return translateQuotaError(err)
	}
	return nil
}

// scanQuotaWindow reads one stored window, including the provider id the list
// query joins in.
func scanQuotaWindow(s scanner) (domain.QuotaWindow, error) {
	var (
		endpointID, providerID, window, source string
		used                                   int64
		limit                                  *int64
		resetsAt                               *time.Time
		updatedAt                              time.Time
	)
	if err := s.Scan(&endpointID, &providerID, &window, &used, &limit, &resetsAt, &source, &updatedAt); err != nil {
		return domain.QuotaWindow{}, err
	}
	return domain.RehydrateQuotaWindow(endpointID, providerID, domain.QuotaWindowKind(window),
		used, limit, resetsAt, domain.QuotaSource(source), updatedAt), nil
}

// translateQuotaError maps a driver error to a domain error a caller can act
// on, wrapping anything unrecognised with table context for the log
// (AGENTS.md §1.3).
func translateQuotaError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrQuotaWindowNotFound
	}
	return fmt.Errorf("quota_windows: %w", err)
}
