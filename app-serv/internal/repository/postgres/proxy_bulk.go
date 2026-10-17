// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/proxy_bulk.go
// @for       The statements a bulk action on proxy candidates runs through: one write shared by the single and the batched path, and the batch delete.
// @uses      context, fmt, github.com/jackc/pgx/v5, internal/domain, internal/repository.
// @reason    Switching a pool off and erasing it are both set operations now, and §2.2 says the store either applied the whole action or applied none of it. updateProxy takes any execer for the same reason the endpoint's write does: one spelling of the UPDATE serves the single route and the batch transaction, instead of two that can start disagreeing about which columns a write touches.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-10-16
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// inTx runs fn in one transaction and rolls back on any error, so a bulk switch or a
// bulk erase cannot leave part of the pool applied.
func (r *ProxyRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("proxies: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx) // reason: the fn error is the outcome the caller acts on; a rollback failure adds nothing.
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("proxies: commit: %w", err)
	}
	return nil
}

// UpdateBulk writes each candidate's own fields in one transaction, so a bulk switch
// lands on every row the operator selected or on none.
func (r *ProxyRepository) UpdateBulk(ctx context.Context, proxies []domain.Proxy) error {
	if len(proxies) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, proxy := range proxies {
			if err := updateProxy(ctx, tx, proxy); err != nil {
				return &BulkRowError{Index: i, Err: err}
			}
		}
		return nil
	})
}

// DeleteBulk removes each candidate in one transaction, reporting the position of the
// row that was not there so a stale selection can be named rather than guessed.
func (r *ProxyRepository) DeleteBulk(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx pgx.Tx) error {
		for i, id := range ids {
			tag, err := tx.Exec(ctx, `DELETE FROM proxies WHERE id = $1`, id)
			if err != nil {
				return &BulkRowError{Index: i, Err: fmt.Errorf("proxies: %w", err)}
			}
			if tag.RowsAffected() == 0 {
				return &BulkRowError{Index: i, Err: domain.ErrProxyNotFound}
			}
		}
		return nil
	})
}

// updateProxy persists one candidate through any execer, including its stored status
// document and its own switch.
func updateProxy(ctx context.Context, exec dbExecer, proxy domain.Proxy) error {
	status, err := encodeProxyStatus(proxy.Status())
	if err != nil {
		return err
	}
	const q = `
UPDATE proxies
SET label = $2, protocol = $3, host = $4, port = $5, username = $6,
	password_encrypted = $7, enabled = $8, status = $9, updated_at = $10
WHERE id = $1`
	tag, err := exec.Exec(ctx, q,
		proxy.ID(), proxy.Label(), string(proxy.Protocol()), proxy.Host(), proxy.Port(),
		proxy.Username(), proxy.PasswordEncrypted(), proxy.Enabled(), status, proxy.UpdatedAt())
	if err != nil {
		return fmt.Errorf("proxies: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProxyNotFound
	}
	return nil
}
