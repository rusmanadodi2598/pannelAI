// Package postgres owns the PostgreSQL connection pool.
//
// @file      internal/repository/postgres/pool.go
// @for       Building the bounded pgx pool used by every repository.
// @uses      context, fmt, time, pgxpool.
// @reason    AGENTS.md §1.7 requires explicit pool limits, so the numbers are set here rather than left to library defaults.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-21
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxConnLifetime = 30 * time.Minute
	maxConnIdleTime = 5 * time.Minute
	healthPeriod    = time.Minute
	minConns        = 2
	pingTimeout     = 5 * time.Second
)

// NewPool parses the DSN, sets every limit explicitly, and verifies the
// connection before returning it to the caller.
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing DATABASE_URL: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.HealthCheckPeriod = healthPeriod

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating the PostgreSQL pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reaching PostgreSQL: %w", err)
	}
	return pool, nil
}
