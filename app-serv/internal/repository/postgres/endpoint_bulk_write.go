// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_bulk_write.go
// @for       The set-sized writes the bulk mutation routes run through: one transaction over accounts, their credentials, and the erasures that take a key set with them.
// @uses      context, github.com/jackc/pgx/v5, internal/domain.
// @reason    The batch routes are all-or-nothing across a resolved set, which no per-aggregate method can promise, so the loops live here and reuse the single-row statements through the pool-or-tx execer rather than restating them. A second spelling of an UPDATE is how two write paths start disagreeing about which columns a write touches, and a bulk action that half-applied is exactly the state this file exists to make impossible. Split from endpoint_batch.go to keep both inside the AGENTS.md §1.1 line budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-16
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// UpdateBulk persists each endpoint's own fields in one transaction, so a bulk action
// lands on every row it resolved or on none. A row that vanished between the resolve and
// the write aborts the set rather than quietly applying to N-1 accounts.
func (r *EndpointRepository) UpdateBulk(ctx context.Context, endpoints []domain.UpstreamEndpoint) error {
	if len(endpoints) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, endpoint := range endpoints {
			if err := updateEndpoint(ctx, tx, endpoint); err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		return nil
	})
}

// UpdateKeysBulk persists each key's own fields in one transaction, on the same
// all-or-nothing terms as the endpoint set above.
func (r *EndpointRepository) UpdateKeysBulk(ctx context.Context, keys []domain.UpstreamKey) error {
	if len(keys) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, key := range keys {
			tagsJSON, err := marshalTags(key.Tags())
			if err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
			if err := updateKey(ctx, tx, key, tagsJSON); err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		return nil
	})
}

// DeleteBulk removes each endpoint in one transaction, its keys going through the
// cascade the schema declares. The rows are deleted one at a time rather than as an
// array so a refusal names the position it belonged to, which is the attribution §8.1
// asks a batch to report.
func (r *EndpointRepository) DeleteBulk(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, id := range ids {
			tag, err := tx.Exec(ctx, `DELETE FROM upstream_endpoints WHERE id = $1`, id)
			if err != nil {
				return &BulkRowError{Index: i, Err: translateEndpointError(err)}
			}
			if tag.RowsAffected() == 0 {
				return &BulkRowError{Index: i, Err: domain.ErrEndpointNotFound}
			}
		}
		return nil
	})
}

// ApplyKeyChanges persists everything one bulk action on credentials moved, in one
// transaction: the accounts whose key sets or stamps changed, the credentials themselves,
// and the credentials that went away.
//
// One statement set per action rather than three calls is what keeps a partial result
// impossible: an operator retagging two hundred keys either sees all of them, or the
// transaction aborts and none of them moved.
func (r *EndpointRepository) ApplyKeyChanges(
	ctx context.Context, endpoints []domain.UpstreamEndpoint, keys []domain.UpstreamKey, deletedKeyIDs []string,
) error {
	if len(endpoints) == 0 && len(keys) == 0 && len(deletedKeyIDs) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, endpoint := range endpoints {
			if err := updateEndpoint(ctx, tx, endpoint); err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		for i, key := range keys {
			tagsJSON, err := marshalTags(key.Tags())
			if err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
			if err := updateKey(ctx, tx, key, tagsJSON); err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		for i, id := range deletedKeyIDs {
			tag, err := tx.Exec(ctx, `DELETE FROM upstream_keys WHERE id = $1`, id)
			if err != nil {
				return &BulkRowError{Index: i, Err: translateKeyError(err)}
			}
			if tag.RowsAffected() == 0 {
				return &BulkRowError{Index: i, Err: domain.NewNotFoundError("upstream key not found")}
			}
		}
		return nil
	})
}
