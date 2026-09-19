// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_status_test.go
// @for       Table-driven tests for the OAuth status report and the forced
//
//	refresh (SPEC-API-001 §7.4 GET .../oauth/status, POST .../oauth/refresh).
//
// @uses      context, testing, time, internal/domain.
// @reason    Status is what the panel reads to decide "connect more accounts"
//
//	or "tokens dying", and the forced refresh is the operator's
//	manual override of the worker; both share the freshness rule,
//	so the table pins that agreement and the grant the refresh
//	sends upstream.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestOAuthStatus(t *testing.T) {
	due := testNow.Add(-time.Minute)
	fresh := testNow.Add(6 * time.Hour)
	cases := []struct {
		name       string
		providerID string
		seed       func(t *testing.T, fixture oauthFlowFixture)
		wantCode   string
		check      func(t *testing.T, status OAuthStatus)
	}{
		{
			name:       "reports per-endpoint refresh state from the provider lead",
			providerID: "identity-provider",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				seedOAuthEndpoint(t, fixture, "ep_due", "identity-provider", "due@example.com", "due@example.com", due)
				seedOAuthEndpoint(t, fixture, "ep_fresh", "identity-provider", "fresh@example.com", "fresh@example.com", fresh)
				seedOAuthEndpoint(t, fixture, "ep_none", "other-provider", "other@example.com", "other@example.com", fresh)
			},
			check: func(t *testing.T, status OAuthStatus) {
				if status.Flow != "code" {
					t.Fatalf("flow = %q, want code", status.Flow)
				}
				if len(status.Endpoints) != 2 {
					t.Fatalf("endpoints = %d, want 2 (other providers excluded)", len(status.Endpoints))
				}
				byID := map[string]OAuthEndpointState{}
				for _, endpoint := range status.Endpoints {
					byID[endpoint.EndpointID] = endpoint
				}
				if got := byID["ep_due"].RefreshState; got != domain.RefreshDue {
					t.Fatalf("ep_due state = %q, want due", got)
				}
				if got := byID["ep_fresh"].RefreshState; got != domain.RefreshFresh {
					t.Fatalf("ep_fresh state = %q, want fresh", got)
				}
				if byID["ep_due"].ExpiresAt == nil {
					t.Fatal("expiry not reported")
				}
			},
		},
		{
			name:       "endpoint without expiry reports missing",
			providerID: "identity-provider",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				endpoint, err := domain.NewUpstreamEndpoint("ep_noexp", "identity-provider", "noexp@example.com", domain.UpstreamAuthOAuth, 1, testNow)
				if err != nil {
					t.Fatalf("seeding: %v", err)
				}
				endpoint.SetOAuth(&domain.OAuthCredential{AccountEmail: "noexp@example.com"}, testNow)
				if err := fixture.store.Create(context.Background(), endpoint); err != nil {
					t.Fatalf("storing: %v", err)
				}
			},
			check: func(t *testing.T, status OAuthStatus) {
				if len(status.Endpoints) != 1 || status.Endpoints[0].RefreshState != domain.RefreshMissing {
					t.Fatalf("states = %+v, want one missing", status.Endpoints)
				}
			},
		},
		{
			name:       "api-key endpoints of the provider are not oauth status",
			providerID: "identity-provider",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				endpoint, err := domain.NewUpstreamEndpoint("ep_key", "identity-provider", "key-account", domain.UpstreamAuthAPIKey, 1, testNow)
				if err != nil {
					t.Fatalf("seeding: %v", err)
				}
				if err := fixture.store.Create(context.Background(), endpoint); err != nil {
					t.Fatalf("storing: %v", err)
				}
			},
			check: func(t *testing.T, status OAuthStatus) {
				if len(status.Endpoints) != 0 {
					t.Fatalf("endpoints = %+v, want none", status.Endpoints)
				}
			},
		},
		{
			name:       "device-flow provider reports its flow kind",
			providerID: "device-provider",
			seed:       func(*testing.T, oauthFlowFixture) {},
			check: func(t *testing.T, status OAuthStatus) {
				if status.Flow != "device" {
					t.Fatalf("flow = %q, want device", status.Flow)
				}
			},
		},
		{
			name:       "connector-required provider reports connector",
			providerID: "connector-provider",
			seed:       func(*testing.T, oauthFlowFixture) {},
			check: func(t *testing.T, status OAuthStatus) {
				if status.Flow != "connector" {
					t.Fatalf("flow = %q, want connector", status.Flow)
				}
			},
		},
		{name: "unknown provider", providerID: "nope", wantCode: "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFlowFixture(t,
				providerWithIdentity("identity-provider"),
				providerWithDeviceFlow("device-provider"),
				providerNeedingConnector("connector-provider"),
			)
			if tc.seed != nil {
				tc.seed(t, fixture)
			}
			status, err := fixture.service.Status(context.Background(), tc.providerID)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Status() error = %v", err)
			}
			tc.check(t, status)
		})
	}
}

func TestOAuthRefresh(t *testing.T) {
	due := testNow.Add(-time.Minute)
	notDue := testNow.Add(6 * time.Hour)
	cases := []struct {
		name       string
		providerID string
		endpointID string
		seed       func(t *testing.T, fixture oauthFlowFixture)
		tokens     func(f *fakeTokenClient)
		wantCode   string
		check      func(t *testing.T, fixture oauthFlowFixture, outcome OAuthRefreshOutcome)
	}{
		{
			name:       "forced single endpoint refreshes regardless of due",
			providerID: "identity-provider",
			endpointID: "ep_fresh",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				seedOAuthEndpoint(t, fixture, "ep_fresh", "identity-provider", "fresh@example.com", "fresh@example.com", notDue)
			},
			check: func(t *testing.T, fixture oauthFlowFixture, outcome OAuthRefreshOutcome) {
				if outcome.Refreshed != 1 || len(outcome.EndpointIDs) != 1 || outcome.EndpointIDs[0] != "ep_fresh" {
					t.Fatalf("outcome = %+v", outcome)
				}
				endpoint, err := fixture.store.GetByID(context.Background(), "ep_fresh")
				if err != nil {
					t.Fatalf("reloading: %v", err)
				}
				opened, err := fixture.sealer.Open(endpoint.OAuth().AccessTokenEncrypted)
				if err != nil || opened != "at-issued" {
					t.Fatalf("access token not replaced: %q (%v)", opened, err)
				}
				if endpoint.OAuth().LastRefreshAt == nil || !endpoint.OAuth().LastRefreshAt.Equal(testNow) {
					t.Fatalf("last refresh = %v, want the service clock", endpoint.OAuth().LastRefreshAt)
				}
				if outcome.ExpiresAt == nil || !outcome.ExpiresAt.Equal(testNow.Add(time.Hour)) {
					t.Fatalf("outcome expiry = %v", outcome.ExpiresAt)
				}
			},
		},
		{
			name:       "empty endpoint id refreshes every due endpoint only",
			providerID: "identity-provider",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				seedOAuthEndpoint(t, fixture, "ep_due", "identity-provider", "due@example.com", "due@example.com", due)
				seedOAuthEndpoint(t, fixture, "ep_later", "identity-provider", "later@example.com", "later@example.com", notDue)
				seedOAuthEndpoint(t, fixture, "ep_other", "other-provider", "other@example.com", "other@example.com", due)
			},
			check: func(t *testing.T, fixture oauthFlowFixture, outcome OAuthRefreshOutcome) {
				if outcome.Refreshed != 1 {
					t.Fatalf("refreshed = %d, want only the due account of this provider", outcome.Refreshed)
				}
				if len(outcome.EndpointIDs) != 1 || outcome.EndpointIDs[0] != "ep_due" {
					t.Fatalf("ids = %v", outcome.EndpointIDs)
				}
			},
		},
		{
			name:       "response refresh token replaces the stored one when rotated",
			providerID: "identity-provider",
			endpointID: "ep_due",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				seedOAuthEndpoint(t, fixture, "ep_due", "identity-provider", "due@example.com", "due@example.com", due)
			},
			tokens: func(f *fakeTokenClient) {
				f.grantFn = func(TokenGrant) (TokenResponse, error) {
					return TokenResponse{AccessToken: "at-2", RefreshToken: "rt-rotated", ExpiresIn: 7200}, nil
				}
			},
			check: func(t *testing.T, fixture oauthFlowFixture, outcome OAuthRefreshOutcome) {
				endpoint, err := fixture.store.GetByID(context.Background(), "ep_due")
				if err != nil {
					t.Fatalf("reloading: %v", err)
				}
				opened, err := fixture.sealer.Open(endpoint.OAuth().RefreshTokenEncrypted)
				if err != nil || opened != "rt-rotated" {
					t.Fatalf("refresh token not rotated: %q (%v)", opened, err)
				}
			},
		},
		{
			name:       "response without a refresh token keeps the stored one",
			providerID: "identity-provider",
			endpointID: "ep_due",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				seedOAuthEndpoint(t, fixture, "ep_due", "identity-provider", "due@example.com", "due@example.com", due)
			},
			tokens: func(f *fakeTokenClient) {
				f.grantFn = func(TokenGrant) (TokenResponse, error) {
					return TokenResponse{AccessToken: "at-2", ExpiresIn: 7200}, nil
				}
			},
			check: func(t *testing.T, fixture oauthFlowFixture, outcome OAuthRefreshOutcome) {
				endpoint, err := fixture.store.GetByID(context.Background(), "ep_due")
				if err != nil {
					t.Fatalf("reloading: %v", err)
				}
				opened, err := fixture.sealer.Open(endpoint.OAuth().RefreshTokenEncrypted)
				if err != nil || opened != "old-refresh" {
					t.Fatalf("refresh token lost: %q (%v)", opened, err)
				}
			},
		},
		{
			name:       "endpoint without a refresh token is refused with a reason",
			providerID: "identity-provider",
			endpointID: "ep_dry",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				access, err := fixture.sealer.Seal("only-access")
				if err != nil {
					t.Fatalf("sealing: %v", err)
				}
				endpoint, err := domain.NewUpstreamEndpoint("ep_dry", "identity-provider", "dry@example.com", domain.UpstreamAuthOAuth, 1, testNow)
				if err != nil {
					t.Fatalf("seeding: %v", err)
				}
				expiry := due
				endpoint.SetOAuth(&domain.OAuthCredential{
					AccessTokenEncrypted: access, ExpiresAt: &expiry, AccountEmail: "dry@example.com",
				}, testNow)
				if err := fixture.store.Create(context.Background(), endpoint); err != nil {
					t.Fatalf("storing: %v", err)
				}
			},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "grant refusal surfaces as upstream error",
			providerID: "identity-provider",
			endpointID: "ep_due",
			seed: func(t *testing.T, fixture oauthFlowFixture) {
				seedOAuthEndpoint(t, fixture, "ep_due", "identity-provider", "due@example.com", "due@example.com", due)
			},
			tokens: func(f *fakeTokenClient) {
				f.grantFn = func(TokenGrant) (TokenResponse, error) {
					return TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant: invalid_grant: token revoked")
				}
			},
			wantCode: "UPSTREAM_ERROR",
		},
		{
			name:       "unknown endpoint id is not found",
			providerID: "identity-provider",
			endpointID: "ep_missing",
			seed:       func(*testing.T, oauthFlowFixture) {},
			wantCode:   "NOT_FOUND",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
			if tc.tokens != nil {
				tc.tokens(fixture.tokens)
			}
			if tc.seed != nil {
				tc.seed(t, fixture)
			}
			outcome, err := fixture.service.Refresh(context.Background(), tc.providerID, tc.endpointID)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
			tc.check(t, fixture, outcome)
		})
	}
}

func TestOAuthRefreshSendsTheRefreshGrant(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	seedOAuthEndpoint(t, fixture, "ep_due", "identity-provider", "due@example.com", "due@example.com", testNow.Add(-time.Minute))
	if _, err := fixture.service.Refresh(context.Background(), "identity-provider", "ep_due"); err != nil {
		t.Fatalf("Refresh(): %v", err)
	}
	if len(fixture.tokens.grantCalls) != 1 {
		t.Fatalf("grant calls = %d, want 1", len(fixture.tokens.grantCalls))
	}
	grant := fixture.tokens.grantCalls[0]
	if grant.GrantType != "refresh_token" {
		t.Fatalf("grant type = %q", grant.GrantType)
	}
	if grant.RefreshToken != "old-refresh" {
		t.Fatalf("refresh token = %q, want the opened stored value", grant.RefreshToken)
	}
	if grant.ClientID != "client-identity-provider" {
		t.Fatalf("client id = %q", grant.ClientID)
	}
}
