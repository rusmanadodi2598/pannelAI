//go:build integration

// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/published_quota_test.go
// @for       Tagged integration test for 000013: the shape the published-quota repository and the quota screen both compile against.
// @uses      database/sql, testing, context.
// @reason    A cache table is the kind of schema change whose failure is a runtime one: a primary key that lost its second column silently accepts a duplicate bucket label, a nullable NOT NULL breaks the sweep's default, an index nobody declared makes the due-sweep a full sort, and a "simplified"
//
//	NOT NULL total turns every unlimited provider allowance into an exhausted
//	one on screen. Each is invisible until a real server parses the file, which
//	is why apply_test.go asserts shape and this file does the same for the new
//	pair rather than trusting the DDL to read correctly.
//
//	  PANNELAI_TEST_POSTGRES_DSN='postgres://...' \
//	    go test -race -tags=integration ./migrations/
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

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

// publishedQuotaShape queries the catalog for one value per case. Each query
// returns text so one scan can serve a count, a nullability flag, and an
// aggregated key.
func TestApply_PublishedQuotaSchemaShape(t *testing.T) {
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
		name string
		sql  string
		args []any
		want string
	}{
		{
			name: "the state table exists",
			sql:  `SELECT (count(*)::text) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`,
			args: []any{"quota_published_state"}, want: "1",
		},
		{
			name: "the window table exists",
			sql:  `SELECT (count(*)::text) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`,
			args: []any{"quota_published_window"}, want: "1",
		},

		// The primary keys are what the batched upsert's ON CONFLICT targets
		// name, and the pair order is what a renamed bucket is keyed by.
		{
			name: "the state key is one endpoint",
			sql:  publishedPrimaryKeySQL, args: []any{"quota_published_state"}, want: "endpoint_id",
		},
		{
			name: "the window key is endpoint plus the provider's own label",
			sql:  publishedPrimaryKeySQL, args: []any{"quota_published_window"}, want: "endpoint_id,label",
		},

		// total is the column a reader will be tempted to make NOT NULL DEFAULT 0.
		// Its nullability IS the unlimited-vs-spent distinction, so it is asserted
		// as a property of the schema, not as a comment.
		{
			name: "a published ceiling may be absent",
			sql:  publishedColumnNullableSQL, args: []any{"quota_published_window", "total"}, want: "YES",
		},
		{
			name: "a published consumption may not",
			sql:  publishedColumnNullableSQL, args: []any{"quota_published_window", "used"}, want: "NO",
		},
		{
			name: "the sweep's ordering column always has an instant",
			sql:  publishedColumnNullableSQL, args: []any{"quota_published_state", "next_attempt_at"}, want: "NO",
		},
		{
			name: "the failure run is always a number",
			sql:  publishedColumnNullableSQL, args: []any{"quota_published_state", "consecutive_failures"}, want: "NO",
		},

		// Both tables die with the account they describe, so an endpoint delete
		// cannot leave a cached quota the screen would keep rendering.
		{
			name: "the state cascades from the endpoint",
			sql:  publishedCascadeSQL, args: []any{"quota_published_state"}, want: "1",
		},
		{
			name: "the windows cascade from the endpoint",
			sql:  publishedCascadeSQL, args: []any{"quota_published_window"}, want: "1",
		},

		// The two indexes are the sweep and the TTL prune, not decoration.
		{
			name: "the due-sweep index exists",
			sql:  `SELECT (count(*)::text) FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1`,
			args: []any{"idx_quota_published_state_due"}, want: "1",
		},
		{
			name: "the TTL prune index exists",
			sql:  `SELECT (count(*)::text) FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1`,
			args: []any{"idx_quota_published_window_fetched"}, want: "1",
		},

		// The CHECK situation is deliberate in both directions: a run length that
		// cannot go negative, and no constraint at all on the window table,
		// because a provider's bucket labels are an unbounded vocabulary, the
		// exact reason this cache is not quota_windows, whose "window" column is
		// CHECKed to four fixed values (000007).
		{
			name: "the failure run cannot go negative",
			sql:  `SELECT (count(*)::text) FROM pg_constraint WHERE conname = $1`,
			args: []any{"quota_published_state_failures_check"}, want: "1",
		},
		{
			name: "no check constrains the provider's label vocabulary",
			sql: `SELECT (count(*)::text) FROM pg_constraint c
			JOIN pg_class t ON t.oid = c.conrelid WHERE t.relname = $1 AND c.contype = 'c'`,
			args: []any{"quota_published_window"}, want: "0",
		},

		// A stored percentage would make a display-rule change a backfill, so the
		// column must not exist at all.
		{
			name: "no percentage is stored",
			sql: `SELECT (count(*)::text) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			  AND column_name IN ('percent', 'percentage', 'used_percent', 'utilization')`,
			args: []any{"quota_published_window"}, want: "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			if err := db.QueryRow(tc.sql, tc.args...).Scan(&got); err != nil {
				t.Fatalf("querying %s: %v", tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("%s: catalog says %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// publishedPrimaryKeySQL returns a table's primary key columns in key order.
const publishedPrimaryKeySQL = `SELECT coalesce(string_agg(kcu.column_name, ',' ORDER BY kcu.ordinal_position), '')
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
WHERE tc.table_schema = 'public' AND tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'`

// publishedColumnNullableSQL reports whether one named column of one named table
// accepts NULL, asked by table and column so the pair can never answer for the
// wrong one.
const publishedColumnNullableSQL = `SELECT is_nullable FROM information_schema.columns
 WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`

// publishedCascadeSQL counts foreign keys from one table to upstream_endpoints
// that carry ON DELETE CASCADE (confdeltype 'c').
const publishedCascadeSQL = `SELECT (count(*)::text) FROM pg_constraint c
JOIN pg_class t ON t.oid = c.conrelid
JOIN pg_class r ON r.oid = c.confrelid
WHERE t.relname = $1 AND r.relname = 'upstream_endpoints' AND c.contype = 'f' AND c.confdeltype = 'c'`
