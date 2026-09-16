// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/migrate.go
// @for       Embedded, ordered, once-only application of the sibling *.up.sql files.
// @uses      embed, io/fs, sort, strings, database/sql, github.com/jackc/pgx/v5/stdlib.
// @reason    SPEC-API-001 §10 puts migrations in P0 and §6 puts them in
//
//	app-serv/migrations. Booting against a database without the schema
//	fails at the first request, which is what a clean machine hits;
//	applying them at boot makes a fresh checkout runnable. A ledger
//	records what ran, so a migration is applied exactly once and a
//	non-idempotent statement stays safe on later boots.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-16
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql
)

// files holds the migration bundle so the binary carries its own schema.
//
//go:embed *.sql
var files embed.FS

// ledgerDDL records which migrations have run. It is created before anything
// else and is itself idempotent.
const ledgerDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    text        PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

// Apply runs every *.up.sql that the ledger has not recorded, in lexical order.
// A migration is applied at most once per database, in its own transaction, so
// a failure leaves the database at the last fully-applied migration and a
// subsequent boot resumes from there.
func Apply(ctx context.Context, dsn string) error {
	names, err := upMigrations()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migrations: opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("migrations: database is unreachable: %w", err)
	}

	if _, err := db.ExecContext(ctx, ledgerDDL); err != nil {
		return fmt.Errorf("migrations: creating ledger: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, name := range names {
		if _, done := applied[name]; done {
			continue
		}
		body, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("migrations: reading %s: %w", name, err)
		}
		if err := applyOne(ctx, db, name, string(body)); err != nil {
			return err
		}
		slog.Info("migration applied", "file", name)
	}
	return nil
}

// appliedVersions reads the ledger into a set.
func appliedVersions(ctx context.Context, db *sql.DB) (applied map[string]struct{}, err error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrations: reading ledger: %w", err)
	}
	// A Close error on this read path would mean the connection is in an
	// unknown state, so it is reported rather than discarded. The named return
	// lets the deferred close reach the caller without shadowing a real error.
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			applied = nil
			err = fmt.Errorf("migrations: closing ledger rows: %w", cerr)
		}
	}()

	applied = make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("migrations: scanning ledger: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrations: iterating ledger: %w", err)
	}
	return applied, nil
}

// applyOne runs a single migration and records it, both inside one transaction:
// either the schema change and its ledger row commit together, or neither does.
func applyOne(ctx context.Context, db *sql.DB, name, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrations: beginning %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, body); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migrations: applying %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)
		 ON CONFLICT (version) DO NOTHING`, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migrations: recording %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrations: committing %s: %w", name, err)
	}
	return nil
}

// upMigrations lists the forward migrations in the order they must run.
func upMigrations() ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("migrations: listing embedded files: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("migrations: no *.up.sql files were embedded")
	}

	// The zero-padded numeric prefix is what orders them; sorting makes that
	// dependency explicit rather than relying on directory listing order.
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names, nil
}
