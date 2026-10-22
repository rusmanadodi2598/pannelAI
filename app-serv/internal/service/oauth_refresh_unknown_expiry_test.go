// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_refresh_unknown_expiry_test.go
// @for       The dead-letter visit for an account whose token carries no expiry.
// @uses      context, testing, internal/domain.
// @reason    A token without an expiry reports RefreshMissing, so gating every sweep on RefreshDue made a grave with that shape unreachable; the ladder window is the only schedule it has.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestOAuthRefreshWorkerRevisitsADeadLetteredAccountWithoutExpiry(t *testing.T) {
	fixture := newWorkerFixture(t, providerWithIdentity("identity-provider"))
	sealer := mustWorkerSealer(t)
	access, err := sealer.Seal("old-access")
	if err != nil {
		t.Fatalf("sealing access token: %v", err)
	}
	refresh, err := sealer.Seal("old-refresh")
	if err != nil {
		t.Fatalf("sealing refresh token: %v", err)
	}
	endpoint, err := domain.NewUpstreamEndpoint("ep_no_expiry", "identity-provider",
		"no-expiry@example.com", domain.UpstreamAuthOAuth, 1, testNow)
	if err != nil {
		t.Fatalf("building endpoint: %v", err)
	}
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted: access, RefreshTokenEncrypted: refresh,
		AccountEmail: "no-expiry@example.com",
	}), testNow)
	endpoint.MarkUnhealthy("the token was refused", testNow)
	endpoint.ScheduleRefreshRetry(testNow)
	if err := fixture.store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing endpoint: %v", err)
	}
	fixture.advance(domain.RefreshRetryBase)

	if refreshed := fixture.worker.Sweep(context.Background()); refreshed != 1 {
		t.Fatalf("sweep refreshed = %d, want the grave visited on its window", refreshed)
	}
	recovered, err := fixture.store.GetByID(context.Background(), "ep_no_expiry")
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if recovered.Status() != domain.UpstreamEndpointActive {
		t.Fatalf("status = %q, want active after a working grant", recovered.Status())
	}
}
