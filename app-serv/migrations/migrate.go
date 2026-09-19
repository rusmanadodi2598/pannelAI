// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/migrate.go
// @for       Embedded, ordered, once-only application of the sibling *.up.sql files.
// @uses      embed, io/fs, sort, strings, database/sql, log/slog, time,
//
//	github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgconn,
//	github.com/jackc/pgx/v5/stdlib.
//
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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
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

// applyLockKey is the advisory-lock key that serialises Apply against one
// database. Every replica of a rolling deploy boots at once, reads the same
// empty ledger, and runs the same CREATE TABLE IF NOT EXISTS — a statement that
// is not atomic against a concurrent creator, so the losers fail with a
// duplicate key on pg_type_typname_nsp_index. One lock over the whole run makes
// a simultaneous start safe.
const applyLockKey int64 = 0x70616e6e656c6169 // "pannelai"

// applyLockWait bounds the wait for that lock, so a replica booting behind a
// stuck migration fails with a readable error instead of hanging until it is
// signalled.
const applyLockWait = 30 * time.Second

// Apply runs every *.up.sql that the ledger has not recorded, in lexical order.
// A migration is applied at most once per database, in its own transaction, so
// a failure leaves the database at the last fully-applied migration and a
// subsequent boot resumes from there.
func Apply(ctx context.Context, dsn string) error {
	names, err := upMigrations()
	if err != nil {
		return err
	}

	db, err := openDB(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("migrations: database is unreachable: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrations: reserving a connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := lockApply(ctx, conn); err != nil {
		return err
	}
	// The unlock outlives a cancelled caller context: releasing the lock is the
	// one statement that must still run while the boot that took it unwinds.
	defer func() {
		if _, err := conn.ExecContext(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, applyLockKey); err != nil {
			slog.Error("migrations: releasing the apply lock failed", "error", err)
		}
	}()

	if _, err := conn.ExecContext(ctx, ledgerDDL); err != nil {
		return fmt.Errorf("migrations: creating ledger: %w", err)
	}

	applied, err := appliedVersions(ctx, conn)
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
		if err := applyOne(ctx, conn, name, string(body)); err != nil {
			return err
		}
		slog.Info("migration applied", "file", name)
	}
	return nil
}

// openDB opens the migration connection with notices routed to the log. A
// migration that cannot do its job reports it with RAISE WARNING, and pgx
// drops notices when OnNotice is nil — a warning nobody reads is the same as
// no warning, so the handler is what makes the refusal visible at boot.
func openDB(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("migrations: reading the DSN: %w", err)
	}
	cfg.OnNotice = func(_ *pgconn.PgConn, notice *pgconn.Notice) {
		// CREATE TABLE IF NOT EXISTS reports "already exists, skipping" as a
		// NOTICE on every boot after the first, so the quiet severities are
		// kept out of the default log and only a real refusal is a warning.
		level := slog.LevelWarn
		switch notice.Severity {
		case "DEBUG", "INFO", "NOTICE", "LOG":
			level = slog.LevelDebug
		}
		slog.Log(context.Background(), level, "migration notice",
			"severity", notice.Severity, "message", notice.Message)
	}
	return stdlib.OpenDB(*cfg), nil
}

// lockApply takes the apply lock, waiting at most applyLockWait.
func lockApply(ctx context.Context, conn *sql.Conn) error {
	lockCtx, cancel := context.WithTimeout(ctx, applyLockWait)
	defer cancel()

	if _, err := conn.ExecContext(lockCtx, `SELECT pg_advisory_lock($1)`, applyLockKey); err != nil {
		return fmt.Errorf("migrations: another replica held the apply lock for %s: %w", applyLockWait, err)
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
