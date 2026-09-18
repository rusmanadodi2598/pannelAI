//go:build integration

// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/apply_lock_test.go
// @for       Tagged integration test for the apply lock that serialises
//
//	concurrent migration runs.
//
// @uses      context, database/sql, strings, sync, testing.
// @reason    Every replica of a rolling deploy runs Apply while it boots, so a
//
//	simultaneous start is the normal case rather than an exotic one. The
//	race it used to hit is a duplicate key on PostgreSQL's internal
//	pg_type_typname_nsp_index, which reads as a schema defect rather than
//	as two replicas colliding — so it needs a test that starts two runs
//	together on purpose.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./migrations/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package migrations

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
)

// scratchSchemaName is fixed rather than generated: only this test uses it, and
// a name that does not move keeps a failed run's leftovers identifiable.
const scratchSchemaName = "migrations_apply_concurrent"

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
