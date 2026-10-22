// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_recovery_test.go
// @for       Tests for how an OAuth account leaves the dead-letter state (SPEC-API-001 §7.4, §7.5).
// @uses      context, domain, oauthhttp, testing, time.
// @reason    The worker stops after a dead-letter on purpose, so the exit is the operator's refresh call, and a status the health system wrote is only ever cleared by a credential that worked. Draft 050 T2 found the account was swept by neither: the worker skipped `error`, so a dead-lettered token read due forever and no path returned it to rotation.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-13
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

// TestOAuthFlowRefreshRecoversADeadLetteredAccount is draft 050 T2's exit. The
// worker deliberately stops after a dead-letter, so recovery comes from the
// refresh call the operator makes: that sweep reaches `error` accounts, and a
// grant the vendor accepts now ends the dead-letter instead of leaving the
// account routable-on-paper only.
func TestOAuthFlowRefreshRecoversADeadLetteredAccount(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_grave", "identity-provider", "grave@example.com", "grave@example.com", testNow.Add(-time.Minute))
	ctx := context.Background()
	fixture.tokens.grantFn = func(oauthhttp.TokenGrant) (oauthhttp.TokenResponse, error) {
		return oauthhttp.TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant: invalid_grant")
	}
	for attempt := 1; attempt <= oauthRefreshMaxAttempts; attempt++ {
		fixture.worker.Sweep(ctx)
		fixture.advance(time.Hour)
	}
	parked, err := fixture.store.GetByID(ctx, "ep_grave")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if parked.Status() != domain.UpstreamEndpointError {
		t.Fatalf("status = %q, want the dead-letter the refresh below ends", parked.Status())
	}

	fixture.tokens.grantFn = nil
	outcome, err := fixture.flow.Refresh(ctx, "identity-provider", "")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if outcome.Refreshed != 1 {
		t.Fatalf("Refreshed = %d, want the dead-lettered account to be reachable by the operator's refresh", outcome.Refreshed)
	}
	recovered, err := fixture.store.GetByID(ctx, "ep_grave")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if recovered.Status() != domain.UpstreamEndpointActive {
		t.Fatalf("status = %q, want active once the credential works again", recovered.Status())
	}
}

// TestOAuthRefreshWorkerRevisitsAndRecoversADeadLetteredAccount is the half of decision
// (ii) the operator path cannot serve: a vendor whose token endpoint was down for the
// whole five-attempt burst buries an account nobody is watching, and the ladder is what
// finds it again. The sweep must give that account one visit when the window passes, and
// a grant that now works must end the dead-letter without an operator involved.
func TestOAuthRefreshWorkerRevisitsAndRecoversADeadLetteredAccount(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_outage", "identity-provider", "outage@example.com", "outage@example.com", testNow.Add(-time.Minute))
	ctx := context.Background()
	fixture.tokens.grantFn = func(oauthhttp.TokenGrant) (oauthhttp.TokenResponse, error) {
		return oauthhttp.TokenResponse{}, domain.NewUpstreamError("the token endpoint is not listening")
	}
	for attempt := 1; attempt <= oauthRefreshMaxAttempts; attempt++ {
		if attempt > 1 {
			fixture.advance(fixture.retryDelay(t, "ep_outage"))
		}
		fixture.worker.Sweep(ctx)
	}
	buried, err := fixture.store.GetByID(ctx, "ep_outage")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if buried.Status() != domain.UpstreamEndpointError {
		t.Fatalf("status = %q, want the burst to bury the account", buried.Status())
	}
	bursts := len(fixture.tokens.grantCalls)

	// The vendor comes back. Nothing has told the gateway, and no operator is looking.
	fixture.tokens.grantFn = nil
	fixture.advance(domain.RefreshRetryBase)
	if refreshed := fixture.worker.Sweep(ctx); refreshed != 1 {
		t.Fatalf("sweep past the window refreshed = %d, want the one visit the ladder allows", refreshed)
	}
	if calls := len(fixture.tokens.grantCalls); calls != bursts+1 {
		t.Fatalf("grant calls = %d, want exactly one visit past the window", calls)
	}

	recovered, err := fixture.store.GetByID(ctx, "ep_outage")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if recovered.Status() != domain.UpstreamEndpointActive {
		t.Fatalf("status = %q, want a working grant to end the dead-letter", recovered.Status())
	}
	if rounds := recovered.RefreshRetryRounds(); rounds != 0 {
		t.Fatalf("retry rounds = %d, want the ladder ended with the grave", rounds)
	}

	// And it goes back to the token's own clock: the next due token is refreshed by
	// the normal path, not by a visit anyone scheduled.
	fixture.advance(domain.RefreshRetryCeiling)
	if refreshed := fixture.worker.Sweep(ctx); refreshed != 1 {
		t.Fatalf("sweep on a recovered account refreshed = %d, want the ordinary due refresh", refreshed)
	}
}

// TestOAuthRefreshWorkerVisitsTheGraveOnItsWindow pins the two halves of the ladder from
// the worker's side: inside the window a dead-lettered account is left alone, and past it
// it gets exactly one grant rather than the burst again (draft 050 T2 decision ii).
//
// This block used to live at the end of TestOAuthRefreshWorkerRetriesThenDeadLetters, where
// it read "a dead-lettered endpoint is not retried on later sweeps" and pinned five grants
// forever. That assertion was the one-way grave this decision replaces, so the burst test now
// stops at the burial and the visits are pinned here, beside the recovery they lead to.
func TestOAuthRefreshWorkerVisitsTheGraveOnItsWindow(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, oauthFlowFixture{store: fixture.store, sealer: mustWorkerSealer(t)},
		"ep_window", "identity-provider", "window@example.com", "window@example.com", testNow.Add(-time.Minute))
	ctx := context.Background()
	fixture.tokens.grantFn = func(oauthhttp.TokenGrant) (oauthhttp.TokenResponse, error) {
		return oauthhttp.TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant: invalid_grant")
	}
	for attempt := 1; attempt <= oauthRefreshMaxAttempts; attempt++ {
		if attempt > 1 {
			fixture.advance(fixture.retryDelay(t, "ep_window"))
		}
		fixture.worker.Sweep(ctx)
	}
	buried, err := fixture.store.GetByID(ctx, "ep_window")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if buried.Status() != domain.UpstreamEndpointError {
		t.Fatalf("status = %q, want the burst to bury the account first", buried.Status())
	}
	bursts := len(fixture.tokens.grantCalls)

	// Inside the window: nothing. A dead-lettered token is expired, so "due" alone
	// would put this account back in every sweep.
	fixture.advance(time.Minute)
	fixture.worker.Sweep(ctx)
	if calls := len(fixture.tokens.grantCalls); calls != bursts {
		t.Fatalf("grant calls = %d, want the account left alone inside its window", calls)
	}

	// Past the window: one visit, still refused, still buried, one step wider.
	fixture.advance(domain.RefreshRetryBase)
	fixture.worker.Sweep(ctx)
	if calls := len(fixture.tokens.grantCalls); calls != bursts+1 {
		t.Fatalf("grant calls = %d, want exactly one revisit past the window, not another burst", calls)
	}
	reloaded, err := fixture.store.GetByID(ctx, "ep_window")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if reloaded.Status() != domain.UpstreamEndpointError {
		t.Fatalf("status = %q, want a failed visit to leave the account buried", reloaded.Status())
	}
	if rounds := reloaded.RefreshRetryRounds(); rounds != 2 {
		t.Fatalf("retry rounds = %d, want the burial plus one failed visit", rounds)
	}
	if until := reloaded.RefreshRetryAt(); until == nil || !until.After(testNow.Add(domain.RefreshRetryBase)) {
		t.Fatalf("window = %v, want the failed visit to have widened it", until)
	}

	// The widened window holds: the next tick, inside it, spends nothing on the grave.
	fixture.advance(time.Minute)
	fixture.worker.Sweep(ctx)
	if calls := len(fixture.tokens.grantCalls); calls != bursts+1 {
		t.Fatalf("grant calls = %d, want the widened window respected on the next tick", calls)
	}
}
