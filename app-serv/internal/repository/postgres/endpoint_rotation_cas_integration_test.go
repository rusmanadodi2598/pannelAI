//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_rotation_cas_integration_test.go
// @for       The compare-and-swap an endpoint write can ask PostgreSQL to perform.
// @uses      context, testing, time, internal/domain
// @reason    An OAuth rotation that raced another must lose instead of storing a token the vendor already replaced. The guard lives in the WHERE clause, so it is pinned against a real row: a service-level double cannot prove the database honours it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-04
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestIntegration_UpdateIfUnchangedRefusesAStaleWrite(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	first := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	second := first.Add(time.Second)

	endpoint, err := domain.NewUpstreamEndpoint(
		"ep-cas", "provider-cas", "label-original", domain.UpstreamAuthOAuth, 1, first,
	)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing endpoint: %v", err)
	}
	loadedAt := endpoint.UpdatedAt()

	// Two writers load the same row. Each changes the label and stamps its own
	// instant, so the stored value says which write landed.
	winner := endpoint
	if err := winner.Update("label-winner", 1, "active", second); err != nil {
		t.Fatalf("winning write mutated the aggregate: %v", err)
	}
	loser := endpoint
	if err := loser.Update("label-loser", 1, "active", second.Add(time.Second)); err != nil {
		t.Fatalf("losing write mutated the aggregate: %v", err)
	}

	if err := repo.UpdateIfUnchanged(ctx, winner, loadedAt); err != nil {
		t.Fatalf("UpdateIfUnchanged() on an unchanged row: %v", err)
	}
	if err := repo.UpdateIfUnchanged(ctx, loser, loadedAt); !isConflict(err) {
		t.Fatalf("stale write error = %v, want CONFLICT", err)
	}

	after, err := repo.GetByID(ctx, "ep-cas")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if after.Label() != "label-winner" {
		t.Fatalf("label = %q, want the winner's: the stale write overwrote the rotation", after.Label())
	}
}

func isConflict(err error) bool {
	return err != nil && domain.AsAppError(err).Code == "CONFLICT"
}
