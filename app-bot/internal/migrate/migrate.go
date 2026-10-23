// Package migrate applies numbered SQL migrations.
//
// @file      internal/migrate/migrate.go
// @for       Applying pending migrations under one advisory lock.
// @uses      context, fmt, pgxpool.
// @reason    Two deploys that race the same migration must not both apply it, so the runner serializes on a PostgreSQL advisory lock and records each version in the same transaction that ran its SQL.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-21
package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockKey int64 = 813_446_271

const createTableSQL = `
CREATE TABLE IF NOT EXISTS bot.schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Up applies every migration whose version is not recorded yet. It holds one
// advisory lock for the whole run, so a second process waits instead of racing.
func Up(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("taking the migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	if _, err := conn.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("creating the migration table: %w", err)
	}

	for _, migration := range migrations {
		var applied bool
		if err := conn.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM bot.schema_migrations WHERE version = $1)",
			migration.Version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("checking migration %06d: %w", migration.Version, err)
		}
		if applied {
			continue
		}
		if err := applyOne(ctx, conn, migration); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, conn *pgxpool.Conn, migration Migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting migration %06d: %w", migration.Version, err)
	}
	if _, err := tx.Exec(ctx, string(migration.UpSQL)); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("applying migration %06d_%s: %w", migration.Version, migration.Name, err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO bot.schema_migrations (version, name) VALUES ($1, $2)",
		migration.Version, migration.Name,
	); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("recording migration %06d: %w", migration.Version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing migration %06d: %w", migration.Version, err)
	}
	return nil
}
