// Package migrations applies the SQL schema app-serv depends on at boot.
//
// @file      migrations/apply_one.go
// @for       Reading the ledger and applying one migration inside one transaction.
// @uses      context, database/sql.
// @reason    The ledger read and the apply-and-record transaction are the two
//
//	statements that make "applied at most once" true; keeping them beside
//	the runner that decides *when* to call them would push migrate.go past
//	the AGENTS.md §1.1 limit, and they change for a different reason: a
//	statement change, not a scheduling change.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

// migrationConn is the statement surface the runner needs. A *sql.Conn
// satisfies it, and taking a connection rather than the pool is what keeps the
// advisory lock and the statements it guards on one session: a session lock
// released on a different connection than it was taken on would not unlock.
type migrationConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// appliedVersions reads the ledger into a set.
func appliedVersions(ctx context.Context, conn migrationConn) (applied map[string]struct{}, err error) {
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
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
func applyOne(ctx context.Context, conn migrationConn, name, body string) error {
	tx, err := conn.BeginTx(ctx, nil)
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
