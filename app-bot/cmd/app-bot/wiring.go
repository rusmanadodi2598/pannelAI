// Command app-bot runs the Telegram media bot service.
//
// @file      cmd/app-bot/wiring.go
// @for       Migration wiring and the HTTP timeouts the server runs with.
// @uses      context, time, internal/migrate, internal/migrations, pgxpool.
// @reason    The migration set is embedded, so applying it is one call the root owns; the timeouts are named here because a deployment tunes them.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-21
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/migrate"
	"github.com/rusmanadodi2598/pannelAI/app-bot/migrations"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	migrationTimeout  = 2 * time.Minute
)

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	list, err := migrate.Parse(migrations.FS, ".")
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()
	if err := migrate.Up(runCtx, pool, list); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}
