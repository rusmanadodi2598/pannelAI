//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_oauth_retry_integration_test.go
// @for       The OAuth dead-letter ladder against a real server: the window and its step surviving a write, a widening visit, and a recovery.
// @uses      context, testing, time, internal/domain.
// @reason    The worker decides whether to visit a buried account from the window it reads back, so a ladder that does not survive the row is a grave with no visiting hours: the account would be attempted on every tick, which is the failure draft 050 T2 decision ii exists to remove. Neither the step arithmetic nor the NULL-versus-zero reading of an unstored deadline is a property the in-memory double can show (AGENTS.md §2.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-13
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

const retryEndpointID = "ep-oauth-retry"

// seedRetryEndpoint stores one OAuth endpoint with no ladder state, the state every
// live account holds.
func seedRetryEndpoint(t *testing.T, repo *EndpointRepository, now time.Time) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		retryEndpointID, "provider-retry", "retry fixture", domain.UpstreamAuthOAuth, 1, now,
	)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	if err := repo.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing endpoint: %v", err)
	}
	stored, err := repo.GetByID(context.Background(), retryEndpointID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	return stored
}

func TestIntegration_OAuthRefreshRetrySurvivesTheRoundTrip(t *testing.T) {
	repo := newEndpointRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	live := seedRetryEndpoint(t, repo, now)
	if live.RefreshRetryAt() != nil || live.RefreshRetryRounds() != 0 {
		t.Fatalf("a fresh account read back as (%v, %d), want no ladder",
			live.RefreshRetryAt(), live.RefreshRetryRounds())
	}
	if !live.RefreshRetryDue(now) {
		t.Fatal("an unstored deadline must read as due, not as a window of zero")
	}

	buried := live
	deadline := buried.ScheduleRefreshRetry(now)
	if err := repo.Update(ctx, buried); err != nil {
		t.Fatalf("persisting the burial: %v", err)
	}

	// The first visit an hour later widens the next one, and both the step and the
	// window have to come back out of the row for that to happen.
	first, err := repo.GetByID(ctx, retryEndpointID)
	if err != nil {
		t.Fatalf("reading the grave back: %v", err)
	}
	if got := first.RefreshRetryRounds(); got != 1 {
		t.Fatalf("rounds = %d, want the burial stored as step 1", got)
	}
	if until := first.RefreshRetryAt(); until == nil || !until.Equal(deadline) {
		t.Fatalf("window = %v, want the stored %v", until, deadline)
	}

	visiting := first
	second := visiting.ScheduleRefreshRetry(deadline)
	if err := repo.Update(ctx, visiting); err != nil {
		t.Fatalf("persisting the failed visit: %v", err)
	}
	revisited, err := repo.GetByID(ctx, retryEndpointID)
	if err != nil {
		t.Fatalf("reading the visit back: %v", err)
	}
	want := deadline.Add(2 * time.Hour)
	if until := revisited.RefreshRetryAt(); until == nil || !until.Equal(want) {
		t.Fatalf("window = %v, want the ladder's second step %v", until, want)
	}
	if !revisited.RefreshRetryDue(second) {
		t.Fatalf("the account must read as due at its own deadline %v", second)
	}

	recovered := revisited
	recovered.MarkHealthy(now)
	recovered.ClearRefreshRetry(now)
	if err := repo.Update(ctx, recovered); err != nil {
		t.Fatalf("persisting the recovery: %v", err)
	}
	cleared, err := repo.GetByID(ctx, retryEndpointID)
	if err != nil {
		t.Fatalf("reading the recovery back: %v", err)
	}
	if cleared.RefreshRetryAt() != nil || cleared.RefreshRetryRounds() != 0 {
		t.Fatalf("recovered ladder = (%v, %d), want both ended by a grant that worked",
			cleared.RefreshRetryAt(), cleared.RefreshRetryRounds())
	}
}
