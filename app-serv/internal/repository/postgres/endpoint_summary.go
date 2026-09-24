// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_summary.go
// @for       The per-provider endpoint roll-up the provider list renders.
// @uses      context, time, internal/domain.
// @reason    SPEC-API-001 §7.4 publishes status_summary for every provider in
//
//	one list response. Reading it per provider would be an N+1 over a page of
//	25 rows (AGENTS.md §1.7 forbids exactly that), so one GROUP BY answers the
//	whole page and the service attaches the counts by provider id.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// EndpointStatusCountsByProvider returns the state roll-up of every endpoint,
// keyed by provider id, in one statement.
//
// The rate-limit state is part of the roll-up because it is what a panel reader
// acts on: an endpoint that is active yet temporarily rate-limited needs to read
// differently from one that is simply healthy, and deriving that distinction
// from a separate query would be a second source of truth for the same fact.
func (r *EndpointRepository) EndpointStatusCountsByProvider(ctx context.Context, providerIDs []string) (map[string]domain.EndpointStatusCounts, error) {
	// An empty id list means "no provider on this page", which is not the same
	// as "every provider": returning everything would answer a question nobody
	// asked and read an unbounded result set.
	if len(providerIDs) == 0 {
		return map[string]domain.EndpointStatusCounts{}, nil
	}

	const query = `
SELECT provider_id, status, (rate_limited_until IS NOT NULL AND rate_limited_until > now()) AS rate_limited, count(*)
  FROM upstream_endpoints
 WHERE provider_id = ANY($1::text[])
 GROUP BY provider_id, status, rate_limited`

	rows, err := r.pool.Query(ctx, query, providerIDs)
	if err != nil {
		return nil, translateEndpointError(err)
	}
	defer rows.Close()

	out := make(map[string]domain.EndpointStatusCounts, len(providerIDs))
	for rows.Next() {
		var providerID, status string
		var rateLimited bool
		var count int64
		if err := rows.Scan(&providerID, &status, &rateLimited, &count); err != nil {
			return nil, translateEndpointError(err)
		}
		summary := out[providerID]
		mergeStatusCount(&summary, domain.UpstreamEndpointStatus(status), rateLimited, count)
		out[providerID] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, translateEndpointError(err)
	}
	return out, nil
}

// mergeStatusCount folds one grouped row into a roll-up, delegating the state
// classification to the domain so the vocabulary has one owner.
func mergeStatusCount(target *domain.EndpointStatusCounts, status domain.UpstreamEndpointStatus, rateLimited bool, count int64) {
	target.Add(status, rateLimited, count)
}

// ActiveProviders returns which of the named providers hold at least one
// endpoint in status active, in one statement. It is the same population the
// data plane's candidates query selects by (`selection.go` narrows
// EndpointFilter{Status: active} per provider), so a read that asks "which
// providers can the router still pick" gets the router's own answer rather
// than an approximation over the roll-up, whose RateLimited bucket is not the
// same set: health tracking moves an endpoint to error without clearing its
// backoff window, so a stale timestamp would count a dead endpoint as pickable.
func (r *EndpointRepository) ActiveProviders(ctx context.Context, providerIDs []string) (map[string]bool, error) {
	if len(providerIDs) == 0 {
		return map[string]bool{}, nil
	}
	const query = `
SELECT DISTINCT provider_id
  FROM upstream_endpoints
 WHERE provider_id = ANY($1::text[]) AND status = 'active'`

	rows, err := r.pool.Query(ctx, query, providerIDs)
	if err != nil {
		return nil, translateEndpointError(err)
	}
	defer rows.Close()

	out := make(map[string]bool, len(providerIDs))
	for rows.Next() {
		var providerID string
		if err := rows.Scan(&providerID); err != nil {
			return nil, translateEndpointError(err)
		}
		out[providerID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, translateEndpointError(err)
	}
	return out, nil
}
