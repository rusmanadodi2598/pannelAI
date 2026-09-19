//go:build integration

// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/ownership_test.go
// @for       Tagged integration test: every table belongs to the schema's app role.
// @uses      context, database/sql, testing.
// @reason    G19 was an ownership defect that only a real server exposes: the
//
//	P2 tables were created by a superuser boot, so the app role could not
//	read them while every other table worked. A test that asserts the
//	invariant across the whole schema catches the next table that lands
//	with the wrong owner, not just the two this migration repaired.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package migrations

import (
	"context"
	"database/sql"
	"testing"
)

// TestApply_EveryTableBelongsToTheAppRole pins the invariant G19 violated: the
// schema has one owning role, taken from the P1 anchor table, and no table
// diverges from it. A superuser-run migration that creates a table would show
// up here as a divergent row rather than as a 500 in the panel.
func TestApply_EveryTableBelongsToTheAppRole(t *testing.T) {
	dsn := testDSN(t)
	if err := Apply(context.Background(), dsn); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer func() { _ = db.Close() }()

	// The anchor is the table the first migration creates, so this reads "the
	// app role" without naming a role that differs per deployment.
	const anchor = `(SELECT pg_get_userbyid(relowner) FROM pg_class
		WHERE oid = 'gateway_keys'::regclass)`

	rows, err := db.Query(`SELECT c.relname, pg_get_userbyid(c.relowner)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		  AND pg_get_userbyid(c.relowner) <> ` + anchor + `
		ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("reading table owners: %v", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var table, owner string
		if err := rows.Scan(&table, &owner); err != nil {
			t.Fatalf("scanning table owners: %v", err)
		}
		t.Errorf("table %s is owned by %s, want the schema's app role", table, owner)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating table owners: %v", err)
	}
}
