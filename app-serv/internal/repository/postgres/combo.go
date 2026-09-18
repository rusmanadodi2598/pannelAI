// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/combo.go
// @for       PostgreSQL persistence for the Combo aggregate root
//
//	(SPEC-API-001 §7.7).
//
// @uses      github.com/jackc/pgx/v5/pgxpool, internal/domain,
//
//	internal/repository.
//
// @reason    A combo's model list is jsonb written whole, because it is part of
//
//	the aggregate: a child table would let a combo be loaded with a
//	list the strategy could never produce. §1.7 forbids a query per
//	row, so the list is one column rather than a join, and the page
//	count comes from a window function in the same statement.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// comboColumns is the projection every read uses, in scan order.
const comboColumns = `id, name, strategy, sticky_limit, judge_model, models, created_at, updated_at`

// ComboRepository persists combos in PostgreSQL.
type ComboRepository struct {
	pool *pgxpool.Pool
}

// NewComboRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewComboRepository(pool *pgxpool.Pool) *ComboRepository {
	return &ComboRepository{pool: pool}
}

// Create inserts a new combo. A duplicate name maps to domain.ErrComboExists so
// callers return 409, not a driver message.
func (r *ComboRepository) Create(ctx context.Context, combo domain.Combo) error {
	const q = `
INSERT INTO combos (id, name, strategy, sticky_limit, judge_model, models, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	models, err := encodeComboModels(combo.Models())
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, q, combo.ID(), combo.Name(), string(combo.Strategy()),
		combo.StickyLimit(), nullString(combo.JudgeModel()), models, combo.CreatedAt(), combo.UpdatedAt())
	if err != nil {
		return translateComboError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewInternalError("combo was not stored")
	}
	return nil
}

// List returns one page of combos ordered by name. The total comes from a window
// function in the same statement, so the count and the page describe one
// snapshot (SPEC-API-001 §4).
func (r *ComboRepository) List(ctx context.Context, q repository.PageQuery) ([]domain.Combo, int64, error) {
	const query = `
SELECT ` + comboColumns + `, count(*) OVER() AS total
  FROM combos
 ORDER BY name ASC
 LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, q.PerPage, q.Offset())
	if err != nil {
		return nil, 0, translateComboError(err)
	}
	defer rows.Close()

	combos := make([]domain.Combo, 0, q.PerPage)
	var total int64
	for rows.Next() {
		combo, rowTotal, err := scanComboWithTotal(rows)
		if err != nil {
			return nil, 0, err
		}
		total = rowTotal
		combos = append(combos, combo)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateComboError(err)
	}

	// A page beyond the last row returns no rows, and therefore no window
	// count; the total is then read separately so pagination still reports it.
	if len(combos) == 0 {
		if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM combos`).Scan(&total); err != nil {
			return nil, 0, translateComboError(err)
		}
	}
	return combos, total, nil
}

// GetByID loads one combo. A missing row yields domain.ErrComboNotFound.
func (r *ComboRepository) GetByID(ctx context.Context, id string) (domain.Combo, error) {
	return r.get(ctx, `SELECT `+comboColumns+` FROM combos WHERE id = $1`, id)
}

// GetByName loads one combo by the name a client sends as a model string
// (§7.15 resolves a combo name first). A missing row yields
// domain.ErrComboNotFound.
func (r *ComboRepository) GetByName(ctx context.Context, name string) (domain.Combo, error) {
	return r.get(ctx, `SELECT `+comboColumns+` FROM combos WHERE name = $1`, name)
}

// ExistsByName reports whether the name is taken, so a create pre-checks and a
// PATCH can keep its own name.
func (r *ComboRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM combos WHERE name = $1)`, name).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking combo name: %w", err)
	}
	return exists, nil
}

// Names returns every combo name, ordered, so validating an alias target or
// resolving a model string needs one query rather than one per reference
// (AGENTS.md §1.7).
func (r *ComboRepository) Names(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT name FROM combos ORDER BY name`)
	if err != nil {
		return nil, translateComboError(err)
	}
	defer rows.Close()

	names := make([]string, 0, 32)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, translateComboError(err)
	}
	return names, nil
}

// Update persists every mutable field, including the model list, as one
// statement so a reader never sees a half-updated combo.
func (r *ComboRepository) Update(ctx context.Context, combo domain.Combo) error {
	const q = `
UPDATE combos
   SET name = $1, strategy = $2, sticky_limit = $3, judge_model = $4, models = $5, updated_at = $6
 WHERE id = $7`

	models, err := encodeComboModels(combo.Models())
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, q, combo.Name(), string(combo.Strategy()), combo.StickyLimit(),
		nullString(combo.JudgeModel()), models, combo.UpdatedAt(), combo.ID())
	if err != nil {
		return translateComboError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrComboNotFound
	}
	return nil
}

// Delete removes one combo.
func (r *ComboRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM combos WHERE id = $1`, id)
	if err != nil {
		return translateComboError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrComboNotFound
	}
	return nil
}

// get runs one single-row read.
func (r *ComboRepository) get(ctx context.Context, query, arg string) (domain.Combo, error) {
	combo, err := scanCombo(r.pool.QueryRow(ctx, query, arg))
	if err != nil {
		return domain.Combo{}, translateComboError(err)
	}
	return combo, nil
}
