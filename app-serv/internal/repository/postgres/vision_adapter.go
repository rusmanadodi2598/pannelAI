// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/vision_adapter.go
// @for       PostgreSQL persistence for the vision adapter configuration
//
//	(SPEC-API-001 §7.8).
//
// @uses      github.com/jackc/pgx/v5, internal/domain, encoding/json.
// @reason    §7.14 keys settings by name and merges defaults at read, so the
//
//	adapter is one settings row holding one typed document. Storing it
//	there rather than in a table of its own means an operator's
//	settings export carries the adapter with everything else, which is
//	what §7.9's round-trip rule already assumes for the token savers.
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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// VisionAdapterSettingsKey is the settings row the adapter configuration lives
// under. It is exported so the settings endpoint can name the same key.
const VisionAdapterSettingsKey = "vision_adapter"

// visionAdapterDocument is the stored JSON shape. It is a decode boundary
// (AGENTS.md §1.4): a concrete struct, never an untyped map, so a document
// written by an older build fails loudly instead of half-decoding.
type visionAdapterDocument struct {
	Enabled    bool      `json:"enabled"`
	RoundRobin bool      `json:"round_robin"`
	Models     []string  `json:"models"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// VisionAdapterRepository stores the adapter configuration in settings.
type VisionAdapterRepository struct {
	pool *pgxpool.Pool
}

// NewVisionAdapterRepository binds the repository to a pool.
func NewVisionAdapterRepository(pool *pgxpool.Pool) *VisionAdapterRepository {
	return &VisionAdapterRepository{pool: pool}
}

// Get returns the stored configuration, or the disabled default when no row
// exists yet: a fresh install must serve a usable shape, not a 404.
func (r *VisionAdapterRepository) Get(ctx context.Context) (domain.VisionAdapter, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, VisionAdapterSettingsKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultVisionAdapter(), nil
	}
	if err != nil {
		return domain.VisionAdapter{}, fmt.Errorf("vision adapter: %w", err)
	}
	var document visionAdapterDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return domain.VisionAdapter{}, fmt.Errorf("vision adapter: decoding settings row: %w", err)
	}
	refs, err := parseAdapterRefs(document.Models)
	if err != nil {
		return domain.VisionAdapter{}, err
	}
	return domain.RehydrateVisionAdapter(document.Enabled, document.RoundRobin, refs, document.UpdatedAt), nil
}

// Save writes the whole configuration as an upsert, so the row's timestamp and
// the reported updated_at are the same write.
func (r *VisionAdapterRepository) Save(ctx context.Context, adapter domain.VisionAdapter) error {
	document := visionAdapterDocument{
		Enabled:    adapter.Enabled(),
		RoundRobin: adapter.RoundRobin(),
		Models:     adapter.ModelStrings(),
		UpdatedAt:  adapter.UpdatedAt(),
	}
	value, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("vision adapter: encoding settings row: %w", err)
	}
	const q = `
INSERT INTO settings (key, value, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`
	if _, err := r.pool.Exec(ctx, q, VisionAdapterSettingsKey, value, adapter.UpdatedAt()); err != nil {
		return fmt.Errorf("vision adapter: %w", err)
	}
	return nil
}

// parseAdapterRefs reads the stored model strings back into references. A
// malformed entry is an error rather than a silent skip: it means the row was
// written by something that did not validate, and serving a shorter list would
// hide that.
func parseAdapterRefs(models []string) ([]domain.ModelRef, error) {
	refs := make([]domain.ModelRef, 0, len(models))
	for _, model := range models {
		ref, err := domain.ParseModelRef(model)
		if err != nil {
			return nil, fmt.Errorf("vision adapter: stored model %q is not provider/model", model)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
