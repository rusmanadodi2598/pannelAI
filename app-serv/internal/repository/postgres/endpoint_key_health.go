// Package postgres implements the repository contracts on PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_key_health.go
// @for       The key's circuit-breaker write after a data-plane outcome.
// @uses      context, internal/domain.
// @reason    The counter has to move inside the database, because two concurrent failures that loaded one starting value would otherwise land as a single counted error and shrink the breaker's backoff. It sits apart from the key CRUD in endpoint_keys.go for the same reason endpoint_parity.go is its own file: a routing result must not rewrite a label, a priority, or a credential an operator edited.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-06
package postgres

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RecordKeyHealth persists a key's circuit-breaker transition after an upstream
// call. It touches health columns only: a routing outcome must not rewrite a
// label, a priority, or the credential it just failed to spend.
//
// The error counter is applied by the database, not written from the aggregate.
// A concurrent pair of failures on one key each loaded the same starting value,
// and two absolute writes would land as one error counted: the breaker would
// under-count and its backoff would shrink. A reset is still written as a reset,
// because clearing to zero twice is the same state as clearing once. An untouched
// key is zero here too, so a caller that writes one clears a count already raised.
func (r *EndpointRepository) RecordKeyHealth(ctx context.Context, key domain.UpstreamKey) error {
	const q = `
UPDATE upstream_keys
   SET status = $1,
       last_used_at = $2,
       last_error = $3,
       consecutive_errors = CASE WHEN $4::boolean THEN 0 ELSE consecutive_errors + 1 END,
       rate_limited_until = $5,
       updated_at = $6
 WHERE id = $7 AND endpoint_id = $8`

	var lastError *string
	if key.LastError() != "" {
		message := key.LastError()
		lastError = &message
	}
	tag, err := r.pool.Exec(ctx, q, string(key.Status()), key.LastUsedAt(), lastError,
		key.ConsecutiveErrors() == 0, key.RateLimitedUntil(), key.UpdatedAt(),
		key.ID(), key.EndpointID())
	if err != nil {
		return translateKeyError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("upstream key not found")
	}
	return nil
}
