// Package repository defines storage contracts consumed by app-serv services.
//
// @file      internal/repository/settings.go
// @for       The SettingsRepository boundary for the typed settings document.
// @uses      context, internal/domain.
// @reason    SPEC-API-001 §7.14 stores settings one row per key so a partial
//
//	PATCH is a single-row upsert and two writers cannot clobber each
//	other's subtree; the service merges the documented defaults over
//	what this boundary returns (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-18
package repository

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SettingsRepository is the storage boundary for the settings document
// (SPEC-API-001 §7.14).
type SettingsRepository interface {
	// Load returns the stored values by key. A key with no row is absent from
	// the map, which the service fills from the documented defaults, so a
	// deployment that never wrote a setting still answers with the §7.14
	// values.
	Load(ctx context.Context) (map[domain.SettingsKey]string, error)

	// Save writes one key's JSON value, replacing whatever was stored. A single
	// key per statement is what keeps two concurrent PATCHes of different
	// groups from overwriting each other.
	Save(ctx context.Context, key domain.SettingsKey, value string) error
}
