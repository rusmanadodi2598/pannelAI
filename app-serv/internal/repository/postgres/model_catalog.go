// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/model_catalog.go
// @for       PostgreSQL persistence for models_custom (SPEC-API-001 §7.6) and
//
//	the driver-error translation the catalog repositories share.
//
// @uses      github.com/jackc/pgx/v5, internal/domain, encoding/json, time.
// @reason    A custom model is a row the operator owns and addresses by id, so
//
//	it persists per row. The capability set is jsonb because it is
//	always read and written whole — it is part of the model, not a
//	table of children — and the driver-error translation lives here
//	because both catalog files must answer a duplicate with the same
//	conflict sentinel.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ModelCatalogRepository persists the catalog's own rows.
type ModelCatalogRepository struct {
	pool *pgxpool.Pool
}

// NewModelCatalogRepository binds the repository to a pool whose limits are set
// explicitly at construction (AGENTS.md §1.7).
func NewModelCatalogRepository(pool *pgxpool.Pool) *ModelCatalogRepository {
	return &ModelCatalogRepository{pool: pool}
}

// Custom returns every custom model, newest first.
func (r *ModelCatalogRepository) Custom(ctx context.Context) ([]domain.CustomModel, error) {
	const q = `
SELECT id, provider_id, model_id, display_name, capabilities, created_at
  FROM models_custom
 ORDER BY created_at DESC, id DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, translateCatalogError(err)
	}
	defer rows.Close()

	models := make([]domain.CustomModel, 0, 32)
	for rows.Next() {
		model, err := scanCustomModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, translateCatalogError(err)
	}
	return models, nil
}

// AddCustom inserts a user-added model. A duplicate (provider_id, model_id)
// maps to domain.ErrModelExists so the service returns 409.
func (r *ModelCatalogRepository) AddCustom(ctx context.Context, model domain.CustomModel) error {
	const q = `
INSERT INTO models_custom (id, provider_id, model_id, display_name, capabilities, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`

	capabilities, err := encodeCapabilities(model.Capabilities())
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, q, model.ID(), model.ProviderID(), model.ModelID(),
		model.DisplayName(), capabilities, model.CreatedAt())
	if err != nil {
		return translateCatalogError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewInternalError("custom model was not stored")
	}
	return nil
}

// RemoveCustom deletes one custom model by id.
func (r *ModelCatalogRepository) RemoveCustom(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM models_custom WHERE id = $1`, id)
	if err != nil {
		return translateCatalogError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrModelNotFound
	}
	return nil
}

// scanCustomModel reads one models_custom row.
func scanCustomModel(s pgx.Rows) (domain.CustomModel, error) {
	var (
		id, providerID, modelID, displayName string
		rawCapabilities                      []byte
		createdAt                            time.Time
	)
	if err := s.Scan(&id, &providerID, &modelID, &displayName, &rawCapabilities, &createdAt); err != nil {
		return domain.CustomModel{}, err
	}
	ref, err := domain.NewModelRef(providerID, modelID)
	if err != nil {
		return domain.CustomModel{}, err
	}
	capabilities, err := decodeCapabilities(rawCapabilities)
	if err != nil {
		return domain.CustomModel{}, err
	}
	return domain.RehydrateCustomModel(id, ref, displayName, capabilities, createdAt), nil
}

// encodeCapabilities renders a capability set as the jsonb array the column
// stores.
func encodeCapabilities(capabilities domain.ModelCapabilities) ([]byte, error) {
	encoded, err := json.Marshal(capabilities.List())
	if err != nil {
		return nil, fmt.Errorf("encoding capabilities: %w", err)
	}
	return encoded, nil
}

// decodeCapabilities reads the jsonb array back into a canonical set.
func decodeCapabilities(raw []byte) (domain.ModelCapabilities, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, fmt.Errorf("decoding capabilities: %w", err)
	}
	return domain.NewModelCapabilities(names...), nil
}

// translateCatalogError maps a driver error to a domain error a caller can act
// on. Unrecognised errors are wrapped with table context; the wrapped text
// reaches logs only, because the handler renders domain.Message, never the
// chain (AGENTS.md §1.3).
func translateCatalogError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrModelNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return fmt.Errorf("model catalog: %w: %s", domain.ErrModelExists, pgErr.ConstraintName)
	}
	return fmt.Errorf("model catalog: %w", err)
}
