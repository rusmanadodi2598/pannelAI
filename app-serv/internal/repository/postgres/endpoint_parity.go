// Package postgres implements the repository contracts on PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_parity.go
// @for       The endpoint's connection-parity write after a data-plane outcome.
// @uses      PostgreSQL connection pool, context, internal/domain.
// @reason    R17 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the parity columns
//
//	migration 000012 added were read by the panel but written by no
//	live path, so the screen could not show why an endpoint last
//	failed. This is the narrow write the data plane's outcome takes,
//	RecordKeyHealth's rule applied to the endpoint's own columns: a
//	routing result must not rewrite a label, a priority, or a
//	credential.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-03
package postgres

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RecordUpstreamOutcome persists the endpoint's connection-parity state after a
// data-plane call: the use run and the last non-test upstream error. It touches
// the parity columns only. A served call carries no error, so its write stores
// NULLs, the screen's "no failure since the last success" state.
func (r *EndpointRepository) RecordUpstreamOutcome(ctx context.Context, endpoint domain.UpstreamEndpoint) error {
	const q = `
UPDATE upstream_endpoints
   SET consecutive_use_count = CASE WHEN $1::boolean THEN 0 ELSE consecutive_use_count + 1 END,
       last_error = $2,
       last_error_at = $3,
       error_code = $4,
       updated_at = $5
 WHERE id = $6`

	code, message, at := endpoint.LastError()
	var lastError, errorCode *string
	if message != "" {
		stored := message
		lastError = &stored
	}
	if code != "" {
		stored := code
		errorCode = &stored
	}
	tag, err := r.pool.Exec(ctx, q, endpoint.ConsecutiveUseCount() == 0,
		lastError, at, errorCode, endpoint.UpdatedAt(), endpoint.ID())
	if err != nil {
		return translateEndpointError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("upstream endpoint not found")
	}
	return nil
}
