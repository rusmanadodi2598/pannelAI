//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_rotation_cas_integration_test.go
// @for       The credential compare-and-swap an endpoint write asks PostgreSQL to perform.
// @uses      context, testing, time, internal/domain
// @reason    An OAuth rotation that raced another must lose instead of storing a token the vendor already replaced, and a rotation that merely raced a served request's bookkeeping write must still land. Both guards live in the WHERE clause, so they are pinned against a real row: a service-level double cannot prove the database honours them.
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

// sealedCredential builds a credential over the given sealed access token, the way
// the sealer's output reaches the aggregate.
func sealedCredential(t *testing.T, access string) *domain.OAuthCredential {
	t.Helper()
	credential, err := domain.NewOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted: access, RefreshTokenEncrypted: access + "-refresh",
	})
	if err != nil {
		t.Fatalf("building credential %q: %v", access, err)
	}
	return credential
}

// storeOAuthEndpoint stores one OAuth account holding the given credential, so a test can
// tell whose write landed by reading the token back.
func storeOAuthEndpoint(
	t *testing.T, repo *EndpointRepository, ctx context.Context, id, access string, at time.Time,
) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, "provider-cas", "label-original",
		domain.UpstreamAuthOAuth, 1, at)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	endpoint.SetOAuth(sealedCredential(t, access), at)
	if err := repo.Create(ctx, endpoint); err != nil {
		t.Fatalf("storing endpoint: %v", err)
	}
	return endpoint
}

// TestIntegration_UpdateIfUnchangedRefusesAStaleCredential is the lost-update guard:
// two writers load one credential, the first rotates it, and the second must lose
// rather than store a token the vendor already replaced.
func TestIntegration_UpdateIfUnchangedRefusesAStaleCredential(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	first := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	second := first.Add(time.Second)

	loaded := storeOAuthEndpoint(t, repo, ctx, "ep-cas", "sealed-shared", first)
	loadedCred := *loaded.OAuth()

	winner := loaded
	winner.SetOAuth(sealedCredential(t, "sealed-winner"), second)
	if err := repo.UpdateIfUnchanged(ctx, winner, loadedCred); err != nil {
		t.Fatalf("UpdateIfUnchanged() on an unchanged credential: %v", err)
	}

	loser := loaded
	loser.SetOAuth(sealedCredential(t, "sealed-loser"), second.Add(time.Second))
	if err := repo.UpdateIfUnchanged(ctx, loser, loadedCred); !isConflict(err) {
		t.Fatalf("stale write error = %v, want CONFLICT", err)
	}

	stored, err := repo.GetByID(ctx, "ep-cas")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got := stored.OAuth().AccessTokenEncrypted(); got != "sealed-winner" {
		t.Fatalf("stored token = %q, want the winner's: the stale write overwrote the rotation", got)
	}
}

// TestIntegration_UpdateIfUnchangedSurvivesAServedWrite is the bug the timestamp
// guard caused: every served request stamps the endpoint's updated_at, so a rotation
// conditioned on it lost to traffic it never raced, and five of those dead-letter an
// account whose credential nobody else had touched.
func TestIntegration_UpdateIfUnchangedSurvivesAServedWrite(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	first := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	servedAt := first.Add(2 * time.Second)

	loaded := storeOAuthEndpoint(t, repo, ctx, "ep-cas-traffic", "sealed-traffic", first)
	loadedCred := *loaded.OAuth()

	// The data plane's own write path: the aggregate records the outcome and the
	// narrow write stores it, credential untouched and timestamp moved.
	served := loaded
	served.RecordUpstreamSuccess(servedAt)
	if err := repo.RecordUpstreamOutcome(ctx, served); err != nil {
		t.Fatalf("recording a served outcome: %v", err)
	}
	moved, err := repo.GetByID(ctx, "ep-cas-traffic")
	if err != nil {
		t.Fatalf("reloading after the served write: %v", err)
	}
	if !moved.UpdatedAt().Equal(servedAt) {
		t.Fatalf("stored updated_at = %v, want the served write's %v", moved.UpdatedAt(), servedAt)
	}

	rotation := loaded
	rotation.SetOAuth(sealedCredential(t, "sealed-rotated"), servedAt.Add(time.Second))
	if err := repo.UpdateIfUnchanged(ctx, rotation, loadedCred); err != nil {
		t.Fatalf("UpdateIfUnchanged() after a served write: %v, want the rotation to keep its guard", err)
	}

	stored, err := repo.GetByID(ctx, "ep-cas-traffic")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got := stored.OAuth().AccessTokenEncrypted(); got != "sealed-rotated" {
		t.Fatalf("stored token = %q, want the rotation's", got)
	}
}

func isConflict(err error) bool {
	return err != nil && domain.AsAppError(err).Code == "CONFLICT"
}
