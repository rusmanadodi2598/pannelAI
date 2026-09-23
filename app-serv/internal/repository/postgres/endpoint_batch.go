// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_batch.go
// @for       The set-scoped statements the bulk routes require: a batch create,
//
//	a batch key append, the sibling priority read, and the OAuth account
//	lookup.
//
// @uses      github.com/jackc/pgx/v5, internal/domain, context.
// @reason    SPEC-API-001 §8.1 makes a batch all-or-nothing, so everything here
//
//	runs inside ONE transaction and returns an error naming the offending
//	row. repository.EndpointRepository is frozen and aggregate-scoped, so
//	these statements extend the concrete type rather than the interface;
//	the service declares the port it needs in endpoint_ports.go. Keeping
//	them in one file is also what keeps the root repository inside the
//	AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// BulkRowError attributes a refused batch to the row that caused it, so the
// service can report {index, error} per row (SPEC-API-001 §8.1) even though
// nothing was written.
//
// It implements BulkRowIndexer structurally, which is how the service recognizes
// it without importing this package.
type BulkRowError struct {
	// Index is the zero-based position of the offending row.
	Index int

	// Err is the reason, already in domain terms.
	Err error
}

// Error renders the cause, so the value reads correctly in a log line.
func (e *BulkRowError) Error() string { return e.Err.Error() }

// Unwrap exposes the cause to errors.Is and errors.As, so a caller matches on the
// domain sentinel rather than on this wrapper.
func (e *BulkRowError) Unwrap() error { return e.Err }

// BulkRowIndex reports the offending row's position.
func (e *BulkRowError) BulkRowIndex() (int, bool) { return e.Index, true }

// CreateBatch persists several endpoints with their keys in one transaction.
//
// The rows are inserted in request order and the first failure aborts the whole
// transaction, which is what makes the batch all-or-nothing: a rejected row
// leaves no account behind, so a half-imported list cannot exist (SPEC-API-001
// §8.1). The offending index is attributed from the position of the statement
// that failed.
func (r *EndpointRepository) CreateBatch(ctx context.Context, endpoints []domain.UpstreamEndpoint) error {
	if len(endpoints) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, endpoint := range endpoints {
			if err := insertEndpoint(ctx, tx, endpoint); err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		return nil
	})
}

// AddKeys appends several keys to one endpoint in one transaction. The endpoint
// is re-checked inside the transaction, so a concurrent delete cannot leave keys
// orphaned between the service's load and this write.
func (r *EndpointRepository) AddKeys(ctx context.Context, endpointID string, keys []domain.UpstreamKey) error {
	if len(keys) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT true FROM upstream_endpoints WHERE id = $1`, endpointID).Scan(&exists); err != nil {
			return translateEndpointError(err)
		}
		return insertKeys(ctx, tx, keys)
	})
}

// IDsByProvider returns every endpoint id of one provider in the priority order
// they currently hold. The ties are broken by id so a renumber started from this
// list is deterministic even when two endpoints share a priority — which the
// reorder exists to resolve, so it must not itself depend on an arbitrary order.
func (r *EndpointRepository) IDsByProvider(ctx context.Context, providerID string) ([]string, error) {
	const q = `SELECT id FROM upstream_endpoints WHERE provider_id = $1 ORDER BY priority, id`

	rows, err := r.pool.Query(ctx, q, providerID)
	if err != nil {
		return nil, translateEndpointError(err)
	}
	defer rows.Close()

	ids := make([]string, 0, 8)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, translateEndpointError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, translateEndpointError(err)
	}
	return ids, nil
}

// FindOAuthEndpoint returns the OAuth endpoint of one provider whose stored
// account identity matches the given email or workspace id.
//
// The comparison runs in SQL against the account jsonb rather than in the
// service, because matching in Go would mean reading every endpoint of the
// provider and hoping the account is on the page — and a re-import that silently
// created a duplicate would be invisible until routing saw two accounts claiming
// one identity.
func (r *EndpointRepository) FindOAuthEndpoint(ctx context.Context, providerID, email, workspaceID string) (string, error) {
	const q = `
SELECT id
  FROM upstream_endpoints
 WHERE provider_id = $1
   AND auth_type = 'oauth'
   AND (($2 <> '' AND account->>'email' = $2)
     OR ($3 <> '' AND account->>'workspace_id' = $3))
 ORDER BY created_at, id
 LIMIT 1`

	var id string
	if err := r.pool.QueryRow(ctx, q, providerID, email, workspaceID).Scan(&id); err != nil {
		return "", translateEndpointError(err)
	}
	return id, nil
}

// insertEndpoint writes one endpoint row and its keys inside the caller's
// transaction. It is shared by Create (single) and CreateBatch, so both paths
// write identical columns — the §8.1 requirement that one create shape serves
// both routes.
func insertEndpoint(ctx context.Context, tx pgx.Tx, endpoint domain.UpstreamEndpoint) error {
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
INSERT INTO upstream_endpoints
    (id, provider_id, label, auth_type, priority, status, oauth, account,
     test_status, rate_limited_until, last_used_at, created_at, updated_at,
     global_priority, default_model, consecutive_use_count,
     last_error, last_error_at, error_code, proxy_pool_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
        $14, $15, $16, $17, $18, $19, $20)`

	args := []any{
		endpoint.ID(), endpoint.ProviderID(), endpoint.Label(),
		string(endpoint.AuthType()), endpoint.Priority(), string(endpoint.Status()),
		oauthJSON, accountJSON, testJSON, endpoint.RateLimitedUntil(),
		endpoint.LastUsedAt(), endpoint.CreatedAt(), endpoint.UpdatedAt(),
	}
	args = append(args, parityColumns(endpoint)...)
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return translateEndpointError(err)
	}
	return insertKeys(ctx, tx, endpoint.Keys())
}
