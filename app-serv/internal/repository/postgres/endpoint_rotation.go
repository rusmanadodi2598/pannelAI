// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_rotation.go
// @for       The selection-time stamp on a key or a keyless endpoint, the fact rotation orders by.
// @uses      PostgreSQL connection pool, context, time.
// @reason    Choosing a credential and serving a call with it are two different moments, and only the second was recorded, so N concurrent selections read one identical timestamp and all picked the same key. This is the narrow write that makes the first moment visible to the next selector, and it is its own statement because a selection has no outcome to report yet: it must not touch a health column, a status, or a window that belongs to a call still being made.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-14
package postgres

import (
	"context"
	"time"
)

// MarkSelected stamps the instant rotation chose a credential. An empty keyID stamps
// the account row: an OAuth or no-auth endpoint carries its credential on the row and
// has no key of its own to advance.
//
// One statement, one column each, so a concurrent selector cannot lose a write it did
// not make: the later stamp wins and both rows advance. A missing row is not an error:
// a key the operator deletes mid-request must not turn bookkeeping into an outage.
//
// A keyed stamp advances the account's column too, because choosing a credential also
// chose the account, and this is the hottest path in the gateway. A data-modifying WITH
// clause runs whether or not the outer statement reads it, so the key is stamped even
// when the account row is already gone.
func (r *EndpointRepository) MarkSelected(ctx context.Context, endpointID, keyID string, at time.Time) error {
	const bothQ = `
WITH chosen AS (
    UPDATE upstream_keys
       SET last_selected_at = $1, updated_at = now()
     WHERE id = $2 AND endpoint_id = $3
)
UPDATE upstream_endpoints
   SET last_selected_at = $1, updated_at = now()
 WHERE id = $3`
	const endpointQ = `
UPDATE upstream_endpoints
   SET last_selected_at = $1, updated_at = now()
 WHERE id = $2`

	var err error
	if keyID != "" {
		_, err = r.pool.Exec(ctx, bothQ, at, keyID, endpointID)
	} else {
		_, err = r.pool.Exec(ctx, endpointQ, at, endpointID)
	}
	if err != nil {
		return translateEndpointError(err)
	}
	return nil
}
