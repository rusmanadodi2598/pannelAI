// Package repository is the storage boundary app-serv services depend on.
//
// @file      internal/repository/media_override.go
// @for       Storage contract for per-kind media provider overrides (SPEC-API-001 §7.10).
// @uses      context, internal/domain.
// @reason    AGENTS.md §1.5 keeps services off the driver. The contract is three
//
//	methods because an override is a small keyed document: the routes
//	need the whole set to render a list, the data plane asks for one
//	row per request, and a save replaces the row — there is no
//	read-then-mutate path that would need a wider read.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package repository

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// MediaOverrideRepository is the storage boundary for media provider overrides
// (SPEC-API-001 §7.10).
type MediaOverrideRepository interface {
	// List returns every stored override. The table holds a handful of rows —
	// one per provider and kind an operator has touched — so it is read whole.
	List(ctx context.Context) ([]domain.MediaOverride, error)

	// Get returns one provider's override for one kind, and whether a row is
	// stored. The data plane asks this per call: it must not read the whole
	// table to answer a question about one key.
	Get(ctx context.Context, providerID string, kind domain.MediaKind) (domain.MediaOverride, bool, error)

	// Upsert writes one override, replacing whatever the row held. A missing
	// row is created: the panel saves a whole form, so "create" and "update"
	// are the same act from its side.
	Upsert(ctx context.Context, override domain.MediaOverride) error
}
