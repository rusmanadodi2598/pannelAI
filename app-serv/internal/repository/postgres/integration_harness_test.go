//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/integration_harness_test.go
// @for       Shared harness for the tagged PostgreSQL integration tests.
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain, internal/migrations,
//
//	context, os, testing, time.
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
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-16
package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// testDSNEnv names the variable that enables these tests.
const testDSNEnv = "PANNELAI_TEST_POSTGRES_DSN"

// newTestPool connects to the configured database and applies migrations, so
// every integration test in this package shares one setup path.
//
// The database's name must declare itself a test database. Every harness in
// this package TRUNCATEs the tables it reads, so pointing the variable at a
// real database destroys its data — which happened once, when a run was aimed
// at the dev database by copying POSTGRES_DSN instead of writing a test DSN:
// the operator's endpoints and keys were truncated away and had to be
// recovered from WAL. A name the guard accepts is one that names itself
// disposable (`pannelai_test`, `pannelai_test_...`); anything else is refused
// before a single statement runs, because a guard that runs after the first
// TRUNCATE is not a guard.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := requireTestDSN(t)

	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newTestRepo returns a gateway-key repository over a clean table.
func newTestRepo(t *testing.T) *GatewayKeyRepository {
	t.Helper()
	pool := newTestPool(t)
	if _, err := pool.Exec(context.Background(), `TRUNCATE gateway_keys`); err != nil {
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

// requireTestDSN reads the integration DSN and refuses one whose database does
// not declare itself a test database, before any statement can run. It is the
// one entry every connection opener in this package calls, so a new harness
// cannot skip the guard by opening its own pool.
func requireTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Fatalf("%s must be set when running with -tags=integration", testDSNEnv)
	}
	if err := guardTestDatabase(dsn); err != nil {
		t.Fatalf("refusing to run integration tests against %q: %v", dsn, err)
	}
	return dsn
}

// guardTestDatabase refuses a DSN whose database name does not declare itself
// a test database. The connection parameters are parsed with the same library
// the pool uses, so a DSN the guard accepts is a DSN the pool can open, and the
// check happens before migrations run — which is what makes it a guard rather
// than an apology.
func guardTestDatabase(dsn string) error {
	parsed, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parsing the DSN: %w", err)
	}
	name := strings.ToLower(parsed.ConnConfig.Database)
	if strings.Contains(name, "test") {
		return nil
	}
	return fmt.Errorf(
		"%q is not a test database (expected a name containing \"test\"; the harness truncates its tables)", name)
}
