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
