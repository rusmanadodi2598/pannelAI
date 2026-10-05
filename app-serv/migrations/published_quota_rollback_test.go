//go:build integration

// The rollback of the published-quota cache is a promise the plan made and no test kept.
//
// @file      migrations/published_quota_rollback_test.go
// @for       Proving 000013's down migration drops both cache tables and that re-applying works.
// @uses      context, database/sql, io/fs, testing.
// @reason    "The down migration runs clean" was verified by hand once, which is not a guarantee
//
// anybody else will honour later: a down file can drift from its up file silently, and a rollback
// that leaves one of the two tables behind is discovered on the way back up, not on the way down.
// This test also pins the ledger step, because the runner skips anything already recorded ,
// dropping tables without clearing the row leaves a schema that re-applies to nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-10-02
package migrations

import (
	"context"
	"database/sql"
	"testing"
)

const publishedQuotaMigration = "000013_published_quota"

func TestPublishedQuotaDownDropsBothTablesAndReapplies(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()

	down, err := files.ReadFile(publishedQuotaMigration + ".down.sql")
	if err != nil {
		t.Fatalf("reading the embedded down migration: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := Apply(ctx, dsn); err != nil {
		t.Fatalf("Apply() before the rollback error = %v", err)
	}
	if got := countPublishedTables(t, db); got != 2 {
		t.Fatalf("published tables after apply = %d, want 2", got)
	}

	if _, err := db.ExecContext(ctx, string(down)); err != nil {
		t.Fatalf("running %s.down.sql: %v", publishedQuotaMigration, err)
	}
	if got := countPublishedTables(t, db); got != 0 {
		t.Fatalf("published tables after the rollback = %d, want 0", got)
	}

	// Re-applying without clearing the ledger must NOT resurrect the tables: that is the trap an
	// operator hits when a rollback is run by hand and the row is left behind.
	if err := Apply(ctx, dsn); err != nil {
		t.Fatalf("Apply() with the ledger row still present error = %v", err)
	}
	if got := countPublishedTables(t, db); got != 0 {
		t.Fatalf("published tables after a skipped re-apply = %d, want 0 (the ledger still held the row)", got)
	}

	if _, err := db.ExecContext(ctx,
		`DELETE FROM schema_migrations WHERE version LIKE $1`, publishedQuotaMigration+"%"); err != nil {
		t.Fatalf("clearing the ledger row: %v", err)
	}
	if err := Apply(ctx, dsn); err != nil {
		t.Fatalf("Apply() after clearing the ledger error = %v", err)
	}
	if got := countPublishedTables(t, db); got != 2 {
		t.Fatalf("published tables after a proper rollback and re-apply = %d, want 2", got)
	}
}

func countPublishedTables(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	err := db.QueryRow(`
		SELECT count(*) FROM pg_tables
		 WHERE schemaname = 'public'
		   AND tablename IN ('quota_published_state', 'quota_published_window')`).Scan(&count)
	if err != nil {
		t.Fatalf("counting the published tables: %v", err)
	}
	return count
}
