// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/usage.go
// @for       Usage recording and the summary aggregation over usage_records.
// @uses      github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    SPEC-API-001 §7.12 answers the summary read from one table and §6
//
//	indexes exactly the group-by dimensions offered. Recording is
//	idempotent on the record's own id so a retried reporter call cannot
//	double-count, and the group-by column is interpolated only from the
//	closed domain set, never from caller text (AGENTS.md §1.7).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// usageRecordColumns is the projection every record read uses, in scan order.
// The nullable columns are coalesced in SQL rather than scanned as pointers:
// an absent endpoint is the empty string, not a distinct null case every caller
// would have to handle.
const usageRecordColumns = `id, request_id, ts, coalesce(endpoint_id, ''), provider_id,
	coalesce(gateway_key_id, ''), model, coalesce(combo, ''), tokens_in, tokens_out,
	tokens_cache_read, tokens_cache_write, cost_usd, latency_ms, status, coalesce(error_code, '')`

// usageFilterClause is the shared FROM and WHERE every usage read appends. An
// empty parameter disables its filter, which lets one statement shape serve
// every combination instead of building SQL from caller input. The free-text
// `q` filter matches a case-insensitive substring of the row's identity or
// error code as well as its model, because the panel's placeholder promises a
// request id and an error code and an operator searching the id on screen must
// not read an empty table (draft 010 F8, owner decision D4 = expand). A NULL
// error_code needs no coalesce: `false OR NULL` excludes a row exactly as
// `false` does, and the other three columns are NOT NULL, so the OR can only
// add a match, never lose one. Keeping the FROM here means a read cannot
// accidentally omit the table and reference bare columns.
const usageFilterClause = `
	  FROM usage_records
	 WHERE ts >= $1 AND ts <= $2
	   AND ($3 = '' OR provider_id = $3)
	   AND ($4 = '' OR endpoint_id = $4)
	   AND ($5 = '' OR model = $5)
	   AND ($6 = '' OR gateway_key_id = $6)
	   AND ($7 = '' OR status = $7)
	   AND ($8 = '' OR request_id ILIKE '%' || $8 || '%'
	                OR id ILIKE '%' || $8 || '%'
	                OR error_code ILIKE '%' || $8 || '%'
	                OR model ILIKE '%' || $8 || '%')`

// usageTotalsProjection is the aggregate every summary, group, and bucket
// selects. cost_usd is cast to text inside the database for the same reason it
// is a string on the wire: summing as numeric stays exact and no float is ever
// constructed (SPEC-API-001 §4).
//
// The percentiles are computed by the database over the same rows, so the whole
// block describes one snapshot. Averaging per-group medians would be a
// different and wrong number, which is why the summary reads its own.
const usageTotalsProjection = `count(*) AS requests,
	coalesce(sum(tokens_in), 0) AS tokens_in,
	coalesce(sum(tokens_out), 0) AS tokens_out,
	coalesce(sum(tokens_cache_read), 0) AS tokens_cache_read,
	coalesce(sum(tokens_cache_write), 0) AS tokens_cache_write,
	coalesce(sum(cost_usd), 0)::numeric(20, 8)::text AS cost_usd,
	coalesce(sum(latency_ms), 0) AS latency_total,
	coalesce(percentile_disc(0.5) WITHIN GROUP (ORDER BY latency_ms), 0)::bigint AS latency_p50,
	coalesce(percentile_disc(0.95) WITHIN GROUP (ORDER BY latency_ms), 0)::bigint AS latency_p95,
	count(*) FILTER (WHERE status <> 'success') AS error_count`

// UsageRepository persists and aggregates usage records.
type UsageRepository struct {
	pool *pgxpool.Pool
}

// NewUsageRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewUsageRepository(pool *pgxpool.Pool) *UsageRepository {
	return &UsageRepository{pool: pool}
}

// Record appends one accounting row.
//
// It is idempotent on the row's own id: recording the same record twice leaves
// one row, so a reporter that retries after a timeout does not double-count.
// Two calls carrying one request_id but different ids are two rows, which is
// intended — a client retrying a gateway request is a second request and is
// billed as one.
func (r *UsageRepository) Record(ctx context.Context, record domain.UsageRecord) error {
	const q = `
INSERT INTO usage_records
    (id, request_id, ts, endpoint_id, provider_id, gateway_key_id, model, combo,
     tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, cost_usd,
     latency_ms, status, error_code)
VALUES ($1, $2, $3, nullif($4, ''), $5, nullif($6, ''), $7, nullif($8, ''),
        $9, $10, $11, $12, $13::numeric, $14, $15, nullif($16, ''))
ON CONFLICT (id) DO NOTHING`

	if _, err := r.pool.Exec(ctx, q,
		record.ID(), record.RequestID(), record.TS(), record.EndpointID(),
		record.ProviderID(), record.GatewayKeyID(), record.Model(), record.Combo(),
		record.TokensIn(), record.TokensOut(), record.TokensCacheRead(),
		record.TokensCacheWrite(), record.CostUSD(), record.LatencyMS(),
		record.Status(), record.ErrorCode()); err != nil {
		return translateUsageError(err)
	}
	return nil
}

// Summary returns the totals and, when asked, one row per dimension value.
//
// The groups come from ONE grouped statement, so the cost of a summary is the
// cost of one scan rather than one query per provider (AGENTS.md §1.7). The
// ungrouped totals are read in a second statement of the same shape: the
// percentiles cannot be recovered from the group rows, and summing them would
// produce a median of medians. The two statements see the same data in practice
// because nothing writes a past timestamp; a caller that needs one snapshot can
// read the totals alone by omitting group_by.
func (r *UsageRepository) Summary(ctx context.Context, filter domain.UsageFilter, groupBy domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	if err := filter.Validate(); err != nil {
		return domain.UsageTotals{}, nil, err
	}
	overall, err := r.totals(ctx, filter)
	if err != nil {
		return domain.UsageTotals{}, nil, err
	}
	if groupBy == "" {
		return overall, nil, nil
	}
	if !groupBy.IsValid() {
		return domain.UsageTotals{}, nil, domain.NewValidationError("invalid group_by: " + string(groupBy))
	}

	// groupBy.Column() comes from the closed UsageGroupBy set and is validated
	// immediately above; PostgreSQL cannot parameterize an identifier, so this
	// is the one place a column name is interpolated and it can never be caller
	// text (AGENTS.md §1.7).
	query := `SELECT coalesce(` + groupBy.Column() + `, '') AS group_key, ` + usageTotalsProjection +
		usageFilterClause + `
	 GROUP BY group_key
	 ORDER BY count(*) DESC, group_key ASC`

	rows, err := r.pool.Query(ctx, query, filterArgs(filter)...)
	if err != nil {
		return domain.UsageTotals{}, nil, translateUsageError(err)
	}
	defer rows.Close()

	groups := make([]domain.UsageGroupRow, 0, 32)
	for rows.Next() {
		row, err := scanGroupedTotals(rows)
		if err != nil {
			return domain.UsageTotals{}, nil, err
		}
		groups = append(groups, row)
	}
	if err := rows.Err(); err != nil {
		return domain.UsageTotals{}, nil, translateUsageError(err)
	}
	return overall, groups, nil
}

// filterArgs renders a filter as the eight positional arguments every read in
// this package shares, so the predicate exists once rather than per caller.
func filterArgs(filter domain.UsageFilter) []any {
	return []any{
		filter.From, filter.To, filter.ProviderID, filter.EndpointID,
		filter.Model, filter.GatewayKey, filter.Status, filter.Query,
	}
}
