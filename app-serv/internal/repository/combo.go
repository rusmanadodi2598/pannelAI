// Package repository is the storage boundary app-serv services depend on.
//
// @file      internal/repository/combo.go
// @for       Storage contracts for combos and their round-robin rotation state.
// @uses      context, internal/domain.
// @reason    AGENTS.md §1.5 keeps services off the driver, and §2.2 makes the
//
//	combo the aggregate root: its model list is jsonb written whole,
//	so the contract exposes the aggregate and never a child row. The
//	rotation state is a separate contract because it lives in Redis
//	(SPEC-API-001 §9.6) while the combo lives in PostgreSQL.
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

// ComboRepository is the storage boundary for combos (SPEC-API-001 §7.7).
type ComboRepository interface {
	// Create persists a new combo. A duplicate name must yield
	// domain.ErrComboExists so the service maps it to CONFLICT.
	Create(ctx context.Context, combo domain.Combo) error

	// List returns one page of combos, ordered by name so the panel's table is
	// stable across pages.
	List(ctx context.Context, q PageQuery) (combos []domain.Combo, total int64, err error)

	// GetByID loads one combo by id. A missing row must yield
	// domain.ErrComboNotFound.
	GetByID(ctx context.Context, id string) (domain.Combo, error)

	// ExistsByName reports whether the name is taken. It exists so a create can
	// pre-check without loading the row, and so a combo may keep its own name
	// through a PATCH.
	ExistsByName(ctx context.Context, name string) (bool, error)

	// Names returns every combo name. A combo is addressed by name in a model
	// string, so alias-target validation and the resolution order of §7.15 need
	// the whole name set at once rather than a row at a time.
	Names(ctx context.Context) ([]string, error)

	// GetByName loads one combo by its name, which is the model string the data
	// plane resolves first (§7.15). A missing row must yield
	// domain.ErrComboNotFound.
	GetByName(ctx context.Context, name string) (domain.Combo, error)

	// Update persists every mutable field, including the model list, as one
	// statement.
	Update(ctx context.Context, combo domain.Combo) error

	// Delete removes one combo. A missing row must yield
	// domain.ErrComboNotFound.
	Delete(ctx context.Context, id string) error
}

// ComboRotationStore persists where a round-robin combo currently sits
// (SPEC-API-001 §9.6 puts sticky round-robin state in Redis).
//
// Next is deliberately the whole operation rather than a load/save pair: two
// concurrent requests for one combo must not both read the same state and then
// both write back the same successor, which a read-modify-write through the
// service cannot prevent. The implementation advances the state atomically and
// returns the order this call should use.
type ComboRotationStore interface {
	// Next returns the model order for this request and advances the stored
	// state under the combo's key. stickyLimit below 1 is treated as 1.
	Next(ctx context.Context, comboKey string, models []string, stickyLimit int) ([]string, error)
}
