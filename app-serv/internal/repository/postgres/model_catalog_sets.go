// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/model_catalog_sets.go
// @for       Whole-set replacement and reads for model_aliases and
//
//	models_disabled (SPEC-API-001 §7.6).
//
// @uses      github.com/jackc/pgx/v5, internal/domain.
// @reason    §7.6 replaces the alias set and the disabled set as a whole, so
//
//	each replacement runs in one transaction: an uncommitted delete is
//	invisible to a concurrent reader, while a delete and an insert in
//	separate statements would let one observe an alias mapping to
//	nothing. §1.7 forbids a query per row, so the insert is a single
//	statement over unnested arrays. These two sets share one shape and
//	one write path, which is why they share one file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"context"
	"fmt"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Aliases returns the whole alias set, ordered by alias.
func (r *ModelCatalogRepository) Aliases(ctx context.Context) ([]domain.ModelAlias, error) {
	rows, err := r.pool.Query(ctx, `SELECT alias, target FROM model_aliases ORDER BY alias`)
	if err != nil {
		return nil, translateCatalogError(err)
	}
	defer rows.Close()

	aliases := make([]domain.ModelAlias, 0, 32)
	for rows.Next() {
		var alias, target string
		if err := rows.Scan(&alias, &target); err != nil {
			return nil, err
		}
		aliases = append(aliases, domain.RehydrateModelAlias(alias, target))
	}
	if err := rows.Err(); err != nil {
		return nil, translateCatalogError(err)
	}
	return aliases, nil
}

// ReplaceAliases swaps the whole set in one transaction (PUT semantics, §7.6).
func (r *ModelCatalogRepository) ReplaceAliases(ctx context.Context, aliases []domain.ModelAlias) error {
	aliasList := make([]string, 0, len(aliases))
	targetList := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		aliasList = append(aliasList, alias.Alias())
		targetList = append(targetList, alias.Target())
	}
	const insert = `
INSERT INTO model_aliases (alias, target)
SELECT * FROM unnest($1::text[], $2::text[])`
	return r.replaceSet(ctx, "model_aliases", "DELETE FROM model_aliases", insert, aliasList, targetList)
}

// Disabled returns every disabled model pair, ordered for a stable response.
func (r *ModelCatalogRepository) Disabled(ctx context.Context) ([]domain.ModelRef, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT provider_id, model_id FROM models_disabled ORDER BY provider_id, model_id`)
	if err != nil {
		return nil, translateCatalogError(err)
	}
	defer rows.Close()

	refs := make([]domain.ModelRef, 0, 32)
	for rows.Next() {
		var providerID, modelID string
		if err := rows.Scan(&providerID, &modelID); err != nil {
			return nil, err
		}
		ref, err := domain.NewModelRef(providerID, modelID)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, translateCatalogError(err)
	}
	return refs, nil
}

// ReplaceDisabled swaps the whole disabled set in one transaction.
func (r *ModelCatalogRepository) ReplaceDisabled(ctx context.Context, refs []domain.ModelRef) error {
	providerList := make([]string, 0, len(refs))
	modelList := make([]string, 0, len(refs))
	for _, ref := range refs {
		providerList = append(providerList, ref.ProviderID())
		modelList = append(modelList, ref.ModelID())
	}
	const insert = `
INSERT INTO models_disabled (provider_id, model_id)
SELECT * FROM unnest($1::text[], $2::text[])`
	return r.replaceSet(ctx, "models_disabled", "DELETE FROM models_disabled", insert, providerList, modelList)
}

// replaceSet runs a whole-set replacement in one transaction: the delete is
// invisible to a concurrent reader until the commit, so no alias ever maps to
// nothing mid-replacement. The insert is one statement over unnested arrays,
// not a statement per row (AGENTS.md §1.7).
func (r *ModelCatalogRepository) replaceSet(ctx context.Context, table, deleteStatement, insertStatement string, args ...any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("replacing %s: %w", table, err)
	}
	defer func() {
		// A rollback after a successful commit is a no-op; its error cannot
		// change what the caller observes.
		_ = tx.Rollback(ctx)
	}()
	if _, err := tx.Exec(ctx, deleteStatement); err != nil {
		return translateCatalogError(err)
	}
	if _, err := tx.Exec(ctx, insertStatement, args...); err != nil {
		return translateCatalogError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("replacing %s: %w", table, err)
	}
	return nil
}
