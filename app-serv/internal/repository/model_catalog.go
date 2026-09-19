// Package repository is the storage boundary app-serv services depend on.
//
// @file      internal/repository/model_catalog.go
// @for       Storage contracts for the model catalog: custom models, the alias
//
//	set, the disabled set, and the vision adapter configuration.
//
// @uses      context, internal/domain.
// @reason    AGENTS.md §1.5 requires services to depend on an interface and
//
//	never on a driver. The alias and disabled sets are read and
//	written whole (SPEC-API-001 §7.6 replaces the full set), so the
//	contract says "set", not "row", and an implementation is free to
//	satisfy it with one statement per set instead of one per row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package repository

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ModelCatalogRepository is the storage boundary for the catalog's own rows
// (SPEC-API-001 §7.6): the models the operator added, the aliases that rename
// them, and the models switched off.
type ModelCatalogRepository interface {
	// Custom returns every custom model, newest first. The set is small by
	// construction — each row is a model an operator typed.
	Custom(ctx context.Context) ([]domain.CustomModel, error)

	// AddCustom persists a new custom model. A duplicate (provider_id,
	// model_id) must yield domain.ErrModelExists so the service maps it to
	// CONFLICT.
	AddCustom(ctx context.Context, model domain.CustomModel) error

	// RemoveCustom deletes one custom model by id. A missing row must yield
	// domain.ErrModelNotFound.
	RemoveCustom(ctx context.Context, id string) error

	// Aliases returns the whole alias set, ordered by alias.
	Aliases(ctx context.Context) ([]domain.ModelAlias, error)

	// ReplaceAliases swaps the entire set in one transaction, so a reader never
	// observes a half-replaced mapping (PUT semantics, §7.6).
	ReplaceAliases(ctx context.Context, aliases []domain.ModelAlias) error

	// Disabled returns every disabled model pair.
	Disabled(ctx context.Context) ([]domain.ModelRef, error)

	// ReplaceDisabled swaps the entire disabled set in one transaction.
	ReplaceDisabled(ctx context.Context, refs []domain.ModelRef) error
}

// VisionAdapterRepository stores the capability fallback configuration
// (SPEC-API-001 §7.8). It is one row in settings, addressed by the key the
// implementation owns.
type VisionAdapterRepository interface {
	// Get returns the stored configuration, or the disabled default when
	// nothing has been written yet: a fresh install must serve a usable shape
	// rather than a 404.
	Get(ctx context.Context) (domain.VisionAdapter, error)

	// Save writes the whole configuration. The row carries the timestamp the
	// response reports, so the write and the reported instant agree.
	Save(ctx context.Context, adapter domain.VisionAdapter) error
}

// VisionRotationStore persists the round-robin position the vision adapter
// continues from, so two image-bearing requests do not always start at the
// same adapter model. The state is advisory: a lost key costs one request of
// skew, never a wrong answer, so the implementation is not required to be
// transactional.
type VisionRotationStore interface {
	// Get returns the stored state, or the zero state when none is stored.
	Get(ctx context.Context) (domain.RotationState, error)

	// Save replaces the stored state.
	Save(ctx context.Context, state domain.RotationState) error
}
