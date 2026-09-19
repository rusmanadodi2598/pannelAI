// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/media_override.go
// @for       PostgreSQL persistence for per-kind media overrides (SPEC-API-001 §7.10).
// @uses      github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgxpool, context, fmt.
// @reason    The row is a keyed document with no relations, so the statements
//
//	are short; the only rule worth stating is that an upsert replaces
//	the whole row, which is what makes a save idempotent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// mediaOverrideColumns is the read column list, in the order scanMediaOverride
// expects.
const mediaOverrideColumns = `provider_id, kind, base_url, default_model, updated_at`

// MediaOverrideRepository stores media provider overrides.
type MediaOverrideRepository struct {
	pool *pgxpool.Pool
}

// NewMediaOverrideRepository binds the repository to a connection pool.
func NewMediaOverrideRepository(pool *pgxpool.Pool) *MediaOverrideRepository {
	return &MediaOverrideRepository{pool: pool}
}

// List returns every override, ordered so a test and a panel read the same
// order twice.
func (r *MediaOverrideRepository) List(ctx context.Context) ([]domain.MediaOverride, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+mediaOverrideColumns+`
		FROM media_provider_settings ORDER BY provider_id, kind`)
	if err != nil {
		return nil, fmt.Errorf("listing media overrides: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MediaOverride, 0)
	for rows.Next() {
		override, err := scanMediaOverride(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, override)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing media overrides: %w", err)
	}
	return out, nil
}

// Get returns one provider's override for one kind. A missing row is not an
// error: "no override stored" is the normal state of a provider nobody has
// configured, and the caller falls back to the registry's own base URL.
func (r *MediaOverrideRepository) Get(ctx context.Context, providerID string, kind domain.MediaKind) (domain.MediaOverride, bool, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+mediaOverrideColumns+`
		FROM media_provider_settings WHERE provider_id = $1 AND kind = $2`,
		providerID, string(kind))
	override, err := scanMediaOverride(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MediaOverride{}, false, nil
	}
	if err != nil {
		return domain.MediaOverride{}, false, err
	}
	return override, true, nil
}

// Upsert writes one override, replacing the row's values when it exists.
func (r *MediaOverrideRepository) Upsert(ctx context.Context, override domain.MediaOverride) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO media_provider_settings
			(provider_id, kind, base_url, default_model, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider_id, kind) DO UPDATE SET
			base_url = EXCLUDED.base_url,
			default_model = EXCLUDED.default_model,
			updated_at = EXCLUDED.updated_at`,
		override.ProviderID(), string(override.Kind()), override.BaseURL(),
		override.DefaultModel(), override.UpdatedAt())
	if err != nil {
		return fmt.Errorf("saving media override: %w", err)
	}
	return nil
}

// scanMediaOverride reads one row into the aggregate.
func scanMediaOverride(row pgx.Row) (domain.MediaOverride, error) {
	var (
		providerID   string
		kind         string
		baseURL      string
		defaultModel string
		updatedAt    time.Time
	)
	if err := row.Scan(&providerID, &kind, &baseURL, &defaultModel, &updatedAt); err != nil {
		return domain.MediaOverride{}, fmt.Errorf("scanning media override: %w", err)
	}
	return domain.RehydrateMediaOverride(providerID, domain.MediaKind(kind), baseURL,
		defaultModel, updatedAt), nil
}
