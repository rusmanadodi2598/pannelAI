//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_delete_by_provider_integration_test.go
// @for       The erase that removes every endpoint of one provider id, with the key cascade behind it.
// @uses      internal/domain, context, errors, testing, time.
// @reason    Deleting a provider node takes its connections with it, and the claim that their stored keys go too rests on one ON DELETE CASCADE the schema declares. No fake can run a constraint, so the cascade, the stop at the provider named, and the empty-provider case are proven against a real server here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-07
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// storeKeyedEndpoint stores one api_key endpoint carrying a single key under providerID.
func storeKeyedEndpoint(
	t *testing.T, repo *EndpointRepository, ctx context.Context, id, providerID string, now time.Time,
) {
	t.Helper()

	endpoint, err := domain.NewUpstreamEndpoint(
		id, providerID, id+"-label", domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("building endpoint %s: %v", id, err)
	}
	if _, err := endpoint.AddKey("primary", "sealed-"+id, "sk-…"+id, 0, now); err != nil {
		t.Fatalf("adding a key to %s: %v", id, err)
	}
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing endpoint %s: %v", id, err)
	}
}

// countStoredKeys reads the key table directly: no repository method returns the keys
// of a missing endpoint, so a row left behind by a missing cascade would be invisible
// through the repository's own reads.
func countStoredKeys(
	t *testing.T, repo *EndpointRepository, ctx context.Context, endpointID string) int {
	t.Helper()

	var count int
	if err := repo.pool.QueryRow(ctx,
		`SELECT count(*) FROM upstream_keys WHERE endpoint_id = $1`, endpointID).Scan(&count); err != nil {
		t.Fatalf("counting the keys of %s: %v", endpointID, err)
	}
	return count
}

func TestIntegration_DeleteByProviderTakesTheEndpointsAndTheirKeys(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)

	storeKeyedEndpoint(t, repo, ctx, "ep-node-1", "provider-node", now)
	storeKeyedEndpoint(t, repo, ctx, "ep-node-2", "provider-node", now)
	storeKeyedEndpoint(t, repo, ctx, "ep-other", "provider-elsewhere", now)

	if err := repo.DeleteByProvider(ctx, "provider-node"); err != nil {
		t.Fatalf("DeleteByProvider(): %v", err)
	}

	for _, id := range []string{"ep-node-1", "ep-node-2"} {
		if _, err := repo.GetByID(ctx, id); !errors.Is(err, domain.ErrEndpointNotFound) {
			t.Fatalf("endpoint %s survived the erase: %v", id, err)
		}
		if left := countStoredKeys(t, repo, ctx, id); left != 0 {
			t.Fatalf("%d key row(s) of %s outlived their endpoint", left, id)
		}
	}

	if _, err := repo.GetByID(ctx, "ep-other"); err != nil {
		t.Fatalf("the erase reached an endpoint of another provider: %v", err)
	}
	if left := countStoredKeys(t, repo, ctx, "ep-other"); left != 1 {
		t.Fatalf("the other provider's key count = %d, want its one key untouched", left)
	}

	// A provider with nothing under it is the ordinary case, so the erase must not
	// report the zero it deleted as a failure.
	if err := repo.DeleteByProvider(ctx, "provider-never-used"); err != nil {
		t.Fatalf("DeleteByProvider() on a provider holding nothing: %v", err)
	}
}
