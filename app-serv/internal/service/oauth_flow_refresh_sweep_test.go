// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_refresh_sweep_test.go
// @for       The rule that a provider-wide refresh sweep passes over disabled accounts.
// @uses      context, testing, time.
// @reason    A disable leaves the stored refresh token live, so the sweep is the only place that can stop a switched-off account spending vendor grants, and an active account must still be refreshed by the same loop.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-16
package service

import (
	"context"
	"testing"
	"time"
)

func TestOAuthSweepGivenDisabledAccountThenNoGrantIsIssued(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	endpoint := seedOAuthEndpoint(t, fixture, "ep_off", "identity-provider", "off@example.com", "off@example.com", testNow.Add(-time.Minute))
	disableEndpoint(t, fixture, endpoint)

	outcome, err := fixture.service.Refresh(context.Background(), "identity-provider", "")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(fixture.tokens.grantCalls) != 0 {
		t.Fatalf("grant calls = %d, want none for a disabled account", len(fixture.tokens.grantCalls))
	}
	if outcome.Refreshed != 0 {
		t.Fatalf("refreshed = %d, want 0", outcome.Refreshed)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0].EndpointID != "ep_off" {
		t.Fatalf("skipped = %+v, want the disabled account named", outcome.Skipped)
	}
}

func TestOAuthSweepGivenTheSameAccountActiveThenItIsRefreshed(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, fixture, "ep_on", "identity-provider", "on@example.com", "on@example.com", testNow.Add(-time.Minute))

	outcome, err := fixture.service.Refresh(context.Background(), "identity-provider", "")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(fixture.tokens.grantCalls) != 1 {
		t.Fatalf("grant calls = %d, want 1", len(fixture.tokens.grantCalls))
	}
	if outcome.Refreshed != 1 || len(outcome.Skipped) != 0 {
		t.Fatalf("outcome = %+v, want one refresh and nothing passed over", outcome)
	}
}
