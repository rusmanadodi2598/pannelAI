//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/integration_harness_test.go
// @for       Shared harness for the tagged PostgreSQL integration tests.
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, internal/repository,
//
//	internal/migrations, testing.
//
// @reason    AGENTS.md §1.7 gives repositories the constraint-level guarantees
//
//	(unique name, bounded pagination), and §2.1 requires that logic be
//	tested. A stub cannot prove a UNIQUE index exists: the earlier P0
//	schema had a plain index on name, so duplicates were accepted by
//	PostgreSQL while the in-memory test double rejected them. These
//	tests run against a real server, which is the only way that
//	divergence surfaces.
//
//	The file carries an `integration` build tag, so the default
//	`go test ./...` does not compile it and stays hermetic (AGENTS.md §2.1
//	forbids t.Skip as a way to sidestep a test). With the tag active the DSN is
//	required, not optional: a missing value fails the test rather than skipping
//	it silently.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./internal/repository/postgres/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     repository
// @stability experimental
// @since     2026-09-16
package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// testDSNEnv names the variable that enables these tests.
const testDSNEnv = "PANNELAI_TEST_POSTGRES_DSN"

// newTestRepo connects to the configured database, applies migrations, and
// returns a repository with a clean table.
func newTestRepo(t *testing.T) *GatewayKeyRepository {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Fatalf("%s must be set when running with -tags=integration", testDSNEnv)
	}

	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE gateway_keys`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return NewGatewayKeyRepository(pool)
}

// seed inserts a key with the given name and returns it.
func seed(t *testing.T, repo *GatewayKeyRepository, name string) domain.GatewayKey {
	t.Helper()
	key := domain.NewGatewayKey(name, "sk-"+name+"-secret-value", "sk-…alue", time.Now().UTC())
	if err := repo.Create(context.Background(), key); err != nil {
		t.Fatalf("seeding %q: %v", name, err)
	}
	return key
}
