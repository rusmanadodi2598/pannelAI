// Package postgres implements the repository contracts on PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_key_health.go
// @for       The key's circuit-breaker write after a data-plane outcome.
// @uses      context, internal/domain.
// @reason    The counter and the window have to move inside the database, because two concurrent failures that loaded one starting value would otherwise land as a single counted error and shorten the breaker's backoff. It sits apart from the key CRUD in endpoint_keys.go for the same reason endpoint_parity.go is its own file: a routing result must not rewrite a label, a priority, or a credential an operator edited.
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
// The counter and the window are computed by the database, not written from the
// aggregate: concurrent selectors read one key at the same count, and absolute
// writes would land one increment and the shorter of two backoffs. A reset stays
// absolute, because a served call is the evidence that releases the park, and an
// untouched key reads zero here too, so a caller that writes one clears a count
// already raised.
func (r *EndpointRepository) RecordKeyHealth(ctx context.Context, key domain.UpstreamKey) error {
	// GREATEST ignores its NULL side, so an unparked key takes the new window
	// without a COALESCE.
	const q = `
UPDATE upstream_keys
   SET status = $1,
       last_used_at = $2,
       last_error = $3,
       consecutive_errors = CASE WHEN $4::boolean THEN 0 ELSE consecutive_errors + 1 END,
       rate_limited_until = CASE WHEN $4::boolean THEN $5::timestamptz
                                 ELSE GREATEST(rate_limited_until, $5::timestamptz) END,
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
