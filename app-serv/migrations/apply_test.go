//go:build integration

// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/apply_test.go
// @for       Tagged integration tests for the migration runner.
// @uses      context, database/sql, os, testing, github.com/jackc/pgx/v5/stdlib.
// @reason    The runner is what makes a fresh checkout boot against an empty
//
//	database, so a migration that fails to parse blocks every deployment
//	while a unit test of the ledger would pass. These tests run the real
//	statements against a real server, which is the only way a reserved
//	word, a CHECK constraint, or a bad index name surfaces.
//
//	The `integration` tag keeps `go test ./...` hermetic (AGENTS.md §2.1
//	forbids t.Skip as a way to sidestep a test); with the tag active the
//	DSN is required, not optional.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./migrations/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package migrations

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

// testDSNEnv names the variable that enables these tests.
const testDSNEnv = "PANNELAI_TEST_POSTGRES_DSN"

// testDSN reads the configured DSN, failing rather than skipping: a silently
// skipped migration test reports safety it never checked.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Fatalf("%s must be set when running with -tags=integration", testDSNEnv)
	}
	return dsn
}

// TestApply_IsIdempotent proves the property the boot path depends on: applying
// twice is not an error, and the second run applies nothing. Without it, a
// migration with a non-idempotent statement would break the second boot of every
// replica.
func TestApply_IsIdempotent(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()

	if err := Apply(ctx, dsn); err != nil {
		t.Fatalf("first Apply() error = %v", err)
	}
	before := appliedRows(t, dsn)

	if err := Apply(ctx, dsn); err != nil {
		t.Fatalf("second Apply() error = %v, want it to be a no-op", err)
	}
	after := appliedRows(t, dsn)

	if after != before {
		t.Fatalf("ledger grew from %d to %d rows on a second Apply, so a migration re-ran", before, after)
	}
}

// TestApply_EveryUpMigrationIsRecorded checks that each embedded migration is in
// the ledger after a run, which is what makes "applied at most once" true rather
// than aspirational.
func TestApply_EveryUpMigrationIsRecorded(t *testing.T) {
	dsn := testDSN(t)
	if err := Apply(context.Background(), dsn); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	names, err := upMigrations()
	if err != nil {
		t.Fatalf("upMigrations() error = %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no migrations were embedded")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, name := range names {
		var found int
		if err := db.QueryRow(
			`SELECT count(*) FROM schema_migrations WHERE version = $1`, name).Scan(&found); err != nil {
			t.Fatalf("reading the ledger for %s: %v", name, err)
		}
		if found != 1 {
			t.Fatalf("migration %s appears %d times in the ledger, want exactly 1", name, found)
		}
	}
}

// TestApply_SchemaHasRequiredObjects checks the shape the repositories depend on,
// including the object whose name needed quoting: a column named `window` is a
// reserved word in PostgreSQL, so it exists only because the migration quotes it.
func TestApply_SchemaHasRequiredObjects(t *testing.T) {
	dsn := testDSN(t)
	if err := Apply(context.Background(), dsn); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = db.Close() }()

	cases := []struct {
		name    string
		query   string
		arg     string
		wantMin int
	}{
		{name: "P1 tables exist", query: `SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1`, arg: "quota_windows", wantMin: 1},
		{name: "the quoted window column exists", query: `SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'quota_windows' AND column_name = $1`, arg: "window", wantMin: 1},
		{name: "the digest lookup index exists", query: `SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'public' AND indexname = $1`, arg: "idx_gateway_keys_value_hash", wantMin: 1},
		{name: "the endpoint key cascade is declared", query: `SELECT count(*) FROM information_schema.table_constraints
			WHERE table_schema = 'public' AND table_name = 'upstream_keys' AND constraint_type = $1`, arg: "FOREIGN KEY", wantMin: 1},
		{name: "the empty window is rejected by a constraint", query: `SELECT count(*) FROM information_schema.check_constraints
			WHERE constraint_name = $1`, arg: "quota_windows_window_check", wantMin: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var found int
			if err := db.QueryRow(tc.query, tc.arg).Scan(&found); err != nil {
				t.Fatalf("query error = %v", err)
			}
			if found < tc.wantMin {
				t.Fatalf("count = %d, want at least %d", found, tc.wantMin)
			}
		})
	}
}

// TestApply_ConcurrentCallsBothSucceed is the regression for a simultaneous
// start: every replica of a rolling deploy runs Apply while it boots, and two
// replicas racing on the same unapplied migration used to fail with a duplicate
// key on PostgreSQL's pg_type_typname_nsp_index. The run happens in its own
// schema so the migrations really are unapplied and the race is real.
func TestApply_ConcurrentCallsBothSucceed(t *testing.T) {
	dsn := scratchSchema(t, testDSN(t))
	ctx := context.Background()

	names, err := upMigrations()
	if err != nil {
		t.Fatalf("upMigrations() error = %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- Apply(ctx, dsn)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Apply() error = %v, want both callers to succeed", err)
		}
	}
	if got := appliedRows(t, dsn); got != len(names) {
		t.Fatalf("ledger holds %d migrations, want %d: a concurrent run must not record one twice",
			got, len(names))
	}
}

// scratchSchema creates a schema of its own and returns a DSN pointed at it, so
// a test that must observe an unapplied database leaves the shared public
// schema — which the repository integration tests run against — untouched.
func scratchSchema(t *testing.T, baseDSN string) string {
	t.Helper()
	ctx := context.Background()

	db, err := sql.Open("pgx", baseDSN)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	// Dropped first so a previous crashed run cannot leave a half-migrated
	// schema behind for this one to mistake for its own.
	if _, err := db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+scratchSchemaName+` CASCADE`); err != nil {
		t.Fatalf("dropping the scratch schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA `+scratchSchemaName); err != nil {
		t.Fatalf("creating the scratch schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+scratchSchemaName+` CASCADE`); err != nil {
			t.Errorf("dropping the scratch schema: %v", err)
		}
		_ = db.Close()
	})

	// search_path is a PostgreSQL runtime parameter, and pgx forwards an
	// unrecognised DSN parameter to the server in the startup packet, so the
	// migrations create their objects in the scratch schema rather than public.
	sep := "?"
	if strings.Contains(baseDSN, "?") {
		sep = "&"
	}
	return baseDSN + sep + "search_path=" + scratchSchemaName
}

// scratchSchemaName is fixed rather than generated: only this test uses it, and
// a name that does not move keeps a failed run's leftovers identifiable.
const scratchSchemaName = "migrations_apply_concurrent"

// appliedRows reports how many migrations the ledger records.
func appliedRows(t *testing.T, dsn string) int {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = db.Close() }()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("reading the ledger: %v", err)
	}
	return count
}
