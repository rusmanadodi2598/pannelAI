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

// RecordKeyHealth persists a key's circuit-breaker transition after an upstream call. It
// touches health columns only: a routing outcome must not rewrite a label, a priority, or
// the credential it failed to spend, and it never rewrites last_selected_at, which belongs
// to the selection that chose the credential (MarkSelected).
//
// The counters and the window move inside the database rather than being written from the
// aggregate: two selectors holding one starting count would otherwise land one increment and
// the shorter backoff. A reset stays absolute because a served call is the evidence that
// releases the park, and each run advances only for the class named, so a run of 429s never
// raises the credential ladder.
func (r *EndpointRepository) RecordKeyHealth(ctx context.Context, key domain.UpstreamKey) error {
	// GREATEST ignores its NULL side, so an unparked key takes the new window without a
	// COALESCE. A served call names no class, so every CASE above takes its reset branch
	// first and a NULL $9 reads as "no column to advance".
	const q = `
UPDATE upstream_keys
   SET status = $1,
       last_used_at = $2,
       last_error = $3,
       consecutive_errors = CASE WHEN $4::boolean THEN 0 ELSE consecutive_errors + 1 END,
       consecutive_auth = CASE WHEN $4::boolean THEN 0
                               WHEN $9::text = 'auth' THEN consecutive_auth + 1
                               ELSE consecutive_auth END,
       consecutive_ratelimit = CASE WHEN $4::boolean THEN 0
                                    WHEN $9::text = 'ratelimit' THEN consecutive_ratelimit + 1
                                    ELSE consecutive_ratelimit END,
       consecutive_billing = CASE WHEN $4::boolean THEN 0
                                  WHEN $9::text = 'billing' THEN consecutive_billing + 1
                                  ELSE consecutive_billing END,
       last_error_class = $9,
       rate_limited_until = CASE WHEN $4::boolean THEN $5::timestamptz
                                 ELSE GREATEST(rate_limited_until, $5::timestamptz) END,
       updated_at = $6
 WHERE id = $7 AND endpoint_id = $8`

	var lastError *string
	if key.LastError() != "" {
		message := key.LastError()
		lastError = &message
	}
	class := failureClassParam(key)
	tag, err := r.pool.Exec(ctx, q, string(key.Status()), key.LastUsedAt(), lastError,
		key.ConsecutiveErrors() == 0, key.RateLimitedUntil(), key.UpdatedAt(),
		key.ID(), key.EndpointID(), class)
	if err != nil {
		return translateKeyError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("upstream key not found")
	}
	return nil
}

// failureClassParam names the run this transition advances, or nothing at all when
// the key served. A stored empty string would read as a class the router does not
// have, so the cleared state is NULL, the same shape last_error uses.
func failureClassParam(key domain.UpstreamKey) *string {
	if key.ConsecutiveErrors() == 0 {
		return nil
	}
	name := key.LastErrorClass()
	if name == "" {
		return nil
	}
	return &name
}
