// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_write.go
// @for       The shared write helpers and the transaction boundary every mutating
//
//	statement in this package runs inside.
//
// @uses      github.com/jackc/pgx/v5, internal/domain, context.
// @reason    Create, CreateBatch, AddKeys, and Reorder all have to write the root and
//
//	its children together, so the transaction boundary and the key-writing
//	statement are declared once here rather than repeated per caller — and
//	one spelling is what makes "the aggregate is saved, or nothing" true on
//	every path (AGENTS.md §2.2). It is separate from endpoint.go because
//	AGENTS.md §1.1 caps a file at 250 lines.
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

// insertKeys writes a created aggregate's keys through COPY, which is one
// statement for the whole set rather than a query per key (AGENTS.md §1.7). An
// empty set is a no-op: an oauth or no_auth endpoint legitimately holds no key.
func insertKeys(ctx context.Context, tx pgx.Tx, keys []domain.UpstreamKey) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"upstream_keys"},
		upstreamKeyCopyColumns,
		pgx.CopyFromSlice(len(keys), func(i int) ([]any, error) {
			return keyCopyRow(keys[i]), nil
		}))
	if err != nil {
		return translateKeyError(err)
	}
	return nil
}

// inTx runs fn in a transaction, rolling back on any error so a partially
// written aggregate cannot survive (AGENTS.md §2.2: the repository saves the
// root, or nothing).
func (r *EndpointRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return translateEndpointError(err)
	}
	if err := fn(tx); err != nil {
		// The rollback error cannot change the outcome the caller acts on: the
		// transaction is discarded either way, so the original failure is what
		// is reported.
		_ = tx.Rollback(ctx) // reason: the fn error is the reported outcome; a rollback failure adds nothing.
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return translateEndpointError(err)
	}
	return nil
}
