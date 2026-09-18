// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/settings.go
// @for       PostgreSQL persistence for the per-key settings document.
// @uses      github.com/jackc/pgx/v5, internal/domain.
// @reason    SPEC-API-001 §6 stores one row per settings key so a partial PATCH
//
//	is a single-row upsert and two writers cannot clobber each other's
//	subtree; the stored value is jsonb so a type change inside a group
//	does not need a migration, and the load is one ranged query rather
//	than a query per key (AGENTS.md §1.7).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SettingsRepository persists the settings document, one row per key.
type SettingsRepository struct {
	pool *pgxpool.Pool
}

// NewSettingsRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

// Load returns every stored settings row keyed by its name, in one query. Rows
// whose key is not one this build knows are skipped rather than failing the
// read: a key written by a newer version, or by the deprecated caveman saver,
// must not stop the panel from reading the settings it does understand.
func (r *SettingsRepository) Load(ctx context.Context) (map[domain.SettingsKey]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value::text FROM settings ORDER BY key ASC`)
	if err != nil {
		return nil, translateSettingsError(err)
	}
	defer rows.Close()

	values := make(map[domain.SettingsKey]string, len(domain.SettingsKeys))
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, translateSettingsError(err)
		}
		values[domain.SettingsKey(key)] = value
	}
	if err := rows.Err(); err != nil {
		return nil, translateSettingsError(err)
	}
	return values, nil
}

// Save writes one key's JSON value, replacing whatever was stored. The value is
// bound as text and cast to jsonb, which means malformed JSON is rejected by
// PostgreSQL rather than stored as a string that reads back wrong.
func (r *SettingsRepository) Save(ctx context.Context, key domain.SettingsKey, value string) error {
	const q = `
INSERT INTO settings (key, value, updated_at)
VALUES ($1, $2::jsonb, now())
ON CONFLICT (key) DO UPDATE
   SET value = EXCLUDED.value, updated_at = now()`

	if _, err := r.pool.Exec(ctx, q, string(key), value); err != nil {
		return translateSettingsError(err)
	}
	return nil
}

// translateSettingsError wraps a driver error with table context for the log.
// No §8 code maps to a settings failure: every one is an internal fault, so the
// wrapped chain reaches logs and never a client body (AGENTS.md §1.3).
func translateSettingsError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("settings: %w", err)
}
