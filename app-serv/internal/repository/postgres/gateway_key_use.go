// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/gateway_key_use.go
// @for       The gateway key use counter: one authenticated call, one row
//
//	update.
//
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, context, time.
// @reason    SPEC-API-001 §7.3 exposes request_count and last_used_at on every
//
//	key, and until now nothing wrote them: the columns read 0 and NULL
//	however many calls a key served (register G6). The increment is an
//	UPDATE ... SET request_count = request_count + 1 rather than a
//	read-modify-write, because two concurrent calls on one key would
//	otherwise lose a count to whichever read finished first.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package postgres

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RecordUse applies one authenticated call to the key: the counter advances and
// last_used_at is stamped in the same statement, so the two columns can never
// describe different calls.
//
// The counter is the database's own arithmetic, not a value read here and
// written back: a key serving concurrent requests is the normal case, and a
// read-modify-write would drop counts under exactly that load.
func (r *GatewayKeyRepository) RecordUse(ctx context.Context, id string, usedAt time.Time) error {
	const q = `
UPDATE gateway_keys
   SET request_count = request_count + 1, last_used_at = $1, updated_at = now()
 WHERE id = $2`

	tag, err := r.pool.Exec(ctx, q, usedAt, id)
	if err != nil {
		return translatePGError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGatewayKeyNotFound
	}
	return nil
}
