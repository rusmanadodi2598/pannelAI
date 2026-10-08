//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/model_catalog_delete_integration_test.go
// @for       The erase that clears one provider's custom and disabled model rows when its node is deleted.
// @uses      migrations, pgxpool, context, testing, time.
// @reason    A node delete takes its endpoints and its model rows with it, and the model half is two statements over two tables that key on the provider id rather than referencing it. Only a real server can prove both tables go, that the pair commits as one transaction, and that a registry provider's own rows in the same columns survive, which is the case a shape-based filter would have deleted.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-08
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// newCatalogPool applies the migrations and empties the two model tables the erase touches.
func newCatalogPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := requireTestDSN(t)
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE models_custom`); err != nil {
		t.Fatalf("truncating models_custom: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE models_disabled`); err != nil {
		t.Fatalf("truncating models_disabled: %v", err)
	}
	return pool
}

func storeCatalogModel(
	t *testing.T, pool *pgxpool.Pool, ctx context.Context, id, providerID, modelID string, now time.Time,
) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO models_custom (id, provider_id, model_id, display_name, capabilities, created_at)
VALUES ($1, $2, $3, $4, '[]', $5)`, id, providerID, modelID, modelID, now)
	if err != nil {
		t.Fatalf("storing custom model %s: %v", id, err)
	}
}

func storeCatalogDisabled(t *testing.T, pool *pgxpool.Pool, ctx context.Context, providerID, modelID string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO models_disabled (provider_id, model_id) VALUES ($1, $2)`, providerID, modelID)
	if err != nil {
		t.Fatalf("disabling %s/%s: %v", providerID, modelID, err)
	}
}

// countCatalogRows runs one fixed query with the provider id bound as a parameter.
func countCatalogRows(
	t *testing.T, pool *pgxpool.Pool, ctx context.Context, query, providerID string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(ctx, query, providerID).Scan(&count); err != nil {
		t.Fatalf("counting with %q for %s: %v", query, providerID, err)
	}
	return count
}

func TestIntegration_DeleteForProviderClearsBothModelTablesAndNothingElse(t *testing.T) {
	pool := newCatalogPool(t)
	repo := NewModelCatalogRepository(pool)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Microsecond)

	storeCatalogModel(t, pool, ctx, "mdl-node-a", "provider-node", "model-a", now)
	storeCatalogModel(t, pool, ctx, "mdl-node-b", "provider-node", "model-b", now)
	storeCatalogModel(t, pool, ctx, "mdl-registry", "opencode", "model-c", now)
	storeCatalogDisabled(t, pool, ctx, "provider-node", "model-a")
	storeCatalogDisabled(t, pool, ctx, "opencode", "model-c")

	if err := repo.DeleteForProvider(ctx, "provider-node"); err != nil {
		t.Fatalf("DeleteForProvider(): %v", err)
	}

	if got := countCatalogRows(t, pool, ctx,
		`SELECT count(*) FROM models_custom WHERE provider_id = $1`, "provider-node"); got != 0 {
		t.Fatalf("custom rows left = %d, want the provider's rows gone", got)
	}
	if got := countCatalogRows(t, pool, ctx,
		`SELECT count(*) FROM models_disabled WHERE provider_id = $1`, "provider-node"); got != 0 {
		t.Fatalf("disabled rows left = %d, want the provider's rows gone", got)
	}

	// The registry provider stores its rows in the same two columns, so the surviving
	// pair is the proof the erase is scoped by the exact id rather than by a shape.
	if got := countCatalogRows(t, pool, ctx,
		`SELECT count(*) FROM models_custom WHERE provider_id = $1`, "opencode"); got != 1 {
		t.Fatalf("registry custom rows = %d, want its one row untouched", got)
	}
	if got := countCatalogRows(t, pool, ctx,
		`SELECT count(*) FROM models_disabled WHERE provider_id = $1`, "opencode"); got != 1 {
		t.Fatalf("registry disabled rows = %d, want its one row untouched", got)
	}

	// A provider with nothing stored is the ordinary case for a fresh node.
	if err := repo.DeleteForProvider(ctx, "provider-never-used"); err != nil {
		t.Fatalf("DeleteForProvider() on a provider holding nothing: %v", err)
	}
}
