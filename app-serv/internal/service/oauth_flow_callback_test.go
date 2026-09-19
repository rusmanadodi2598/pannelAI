// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_callback_test.go
// @for       Table-driven tests for completing an authorization (SPEC-API-001
//
//	§7.4 GET .../oauth/callback): replay, exchange, identity
//	matching, and the sealing rule.
//
// @uses      context, strings, testing, time, internal/domain.
// @reason    The callback is the only place tokens enter the system from a
//
//	provider, so the table pins the replay guard, the fail-closed
//	identity rule, the update-not-duplicate account rule, and that no
//	stored field ever carries plaintext token material.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// startFlow drives a real Start so every callback test stages its state the
// same way production would, rather than hand-crafting a payload.
func startFlow(t *testing.T, fixture oauthFlowFixture, providerID string) string {
	t.Helper()
	started, err := fixture.service.Start(context.Background(), OAuthStartInput{
		ProviderID: providerID, BaseURL: "https://panel.example.com",
	})
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	return started.State
}

func TestOAuthCallback(t *testing.T) {
	cases := []struct {
		name       string
		providerID string
		setup      func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput
		tokens     func(f *fakeTokenClient)
		wantCode   string
		check      func(t *testing.T, fixture oauthFlowFixture, result OAuthConnect)
	}{
		{
			name:       "first connect creates an endpoint and seals both tokens",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "identity-provider", Code: "the-code", State: state}
			},
			tokens: func(f *fakeTokenClient) {
				f.infoFn = func() (OAuthIdentity, error) {
					return OAuthIdentity{Sub: "sub-123", Email: "dev@example.com", Name: "Dev One"}, nil
				}
			},
			check: func(t *testing.T, fixture oauthFlowFixture, result OAuthConnect) {
				if !result.Created {
					t.Fatal("want created=true")
				}
				endpoint := result.Endpoint
				if endpoint.AuthType() != domain.UpstreamAuthOAuth {
					t.Fatalf("auth type = %q", endpoint.AuthType())
				}
				if endpoint.Account().Email != "dev@example.com" {
					t.Fatalf("account email = %q", endpoint.Account().Email)
				}
				if endpoint.Label() != "dev@example.com" {
					t.Fatalf("label = %q, want the identity", endpoint.Label())
				}
				cred := endpoint.OAuth()
				if cred == nil {
					t.Fatal("no credential stored")
				}
				if strings.Contains(cred.AccessTokenEncrypted, "at-issued") {
					t.Fatal("access token stored as plaintext")
				}
				if strings.Contains(cred.RefreshTokenEncrypted, "rt-issued") {
					t.Fatal("refresh token stored as plaintext")
				}
				opened, err := fixture.sealer.Open(cred.AccessTokenEncrypted)
				if err != nil || opened != "at-issued" {
					t.Fatalf("sealed access token does not open to the issued value: %q (%v)", opened, err)
				}
				if result.TokenHint != domain.MaskSecret("at-issued") {
					t.Fatalf("token hint = %q", result.TokenHint)
				}
				if cred.ExpiresAt == nil || !cred.ExpiresAt.Equal(testNow.Add(time.Hour)) {
					t.Fatalf("expiry = %v, want now+expires_in", cred.ExpiresAt)
				}
				if result.RedirectBase != "https://panel.example.com" {
					t.Fatalf("redirect base = %q", result.RedirectBase)
				}
			},
		},
		{
			name:       "reconnect of a known account updates its endpoint in place",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				seedOAuthEndpoint(t, fixture, "ep_existing", "identity-provider", "dev@example.com", "dev@example.com", testNow.Add(-time.Hour))
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "identity-provider", Code: "the-code", State: state}
			},
			tokens: func(f *fakeTokenClient) {
				f.infoFn = func() (OAuthIdentity, error) {
					return OAuthIdentity{Sub: "sub-123", Email: "dev@example.com"}, nil
				}
			},
			check: func(t *testing.T, fixture oauthFlowFixture, result OAuthConnect) {
				if result.Created {
					t.Fatal("want created=false for a known account")
				}
				if result.Endpoint.ID() != "ep_existing" {
					t.Fatalf("endpoint id = %q, want the existing account updated", result.Endpoint.ID())
				}
				opened, err := fixture.sealer.Open(result.Endpoint.OAuth().AccessTokenEncrypted)
				if err != nil || opened != "at-issued" {
					t.Fatalf("token not replaced: %q (%v)", opened, err)
				}
				count := 0
				for _, endpoint := range fixture.store.byID {
					if endpoint.ProviderID() == "identity-provider" {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("provider has %d endpoints, want 1", count)
				}
			},
		},
		{
			name:       "provider without userinfo still connects under a default label",
			providerID: "pkce-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "pkce-provider")
				return OAuthCallbackInput{ProviderID: "pkce-provider", Code: "the-code", State: state}
			},
			check: func(t *testing.T, fixture oauthFlowFixture, result OAuthConnect) {
				if !result.Created {
					t.Fatal("want created=true")
				}
				if result.Endpoint.Label() != "pkce-provider oauth" {
					t.Fatalf("label = %q", result.Endpoint.Label())
				}
				if fixture.tokens.infoCalls != 0 {
					t.Fatalf("userinfo called %d times without a declared endpoint", fixture.tokens.infoCalls)
				}
			},
		},
		{
			name:       "replayed state is refused",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				input := OAuthCallbackInput{ProviderID: "identity-provider", Code: "first", State: state}
				if _, err := fixture.service.Callback(context.Background(), input); err != nil {
					t.Fatalf("first callback: %v", err)
				}
				return input
			},
			wantCode: "VALIDATION_ERROR",
		},
		{name: "unknown state is refused", providerID: "identity-provider",
			setup: func(*testing.T, oauthFlowFixture) OAuthCallbackInput {
				return OAuthCallbackInput{ProviderID: "identity-provider", Code: "c", State: "never-staged"}
			},
			wantCode: "VALIDATION_ERROR"},
		{
			name:       "state bound to another provider is refused",
			providerID: "pkce-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "pkce-provider", Code: "c", State: state}
			},
			wantCode: "VALIDATION_ERROR",
		},
		{
			name:       "provider-reported error becomes an upstream error",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "identity-provider", State: state,
					Error: "access_denied", ErrorDescription: "the user declined"}
			},
			wantCode: "UPSTREAM_ERROR",
		},
		{
			name:       "token endpoint refusal is an upstream error",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "identity-provider", Code: "bad-code", State: state}
			},
			tokens: func(f *fakeTokenClient) {
				f.grantFn = func(TokenGrant) (TokenResponse, error) {
					return TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant: invalid_grant: code expired")
				}
			},
			wantCode: "UPSTREAM_ERROR",
		},
		{
			name:       "declared userinfo failing fails closed and writes nothing",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "identity-provider", Code: "the-code", State: state}
			},
			tokens: func(f *fakeTokenClient) {
				f.infoFn = func() (OAuthIdentity, error) {
					return OAuthIdentity{}, domain.NewUpstreamError("the user info endpoint rejected the request")
				}
			},
			wantCode: "UPSTREAM_ERROR",
		},
		{
			name:       "missing code with a valid state is a validation error",
			providerID: "identity-provider",
			setup: func(t *testing.T, fixture oauthFlowFixture) OAuthCallbackInput {
				state := startFlow(t, fixture, "identity-provider")
				return OAuthCallbackInput{ProviderID: "identity-provider", State: state}
			},
			wantCode: "VALIDATION_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFlowFixture(t, providerWithCodeFlow("pkce-provider"), providerWithIdentity("identity-provider"))
			if tc.tokens != nil {
				tc.tokens(fixture.tokens)
			}
			input := tc.setup(t, fixture)
			result, err := fixture.service.Callback(context.Background(), input)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Callback() error = %v", err)
			}
			tc.check(t, fixture, result)
		})
	}
}

func TestOAuthCallbackRefusesUnknownProvider(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	_, err := fixture.service.Callback(context.Background(),
		OAuthCallbackInput{ProviderID: "nope", Code: "c", State: "s"})
	mustAppError(t, err, "VALIDATION_ERROR")
}

func TestOAuthCallbackExchangesTheStagedRedirect(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithIdentity("identity-provider"))
	state := startFlow(t, fixture, "identity-provider")
	if _, err := fixture.service.Callback(context.Background(),
		OAuthCallbackInput{ProviderID: "identity-provider", Code: "the-code", State: state}); err != nil {
		t.Fatalf("Callback(): %v", err)
	}
	if len(fixture.tokens.grantCalls) != 1 {
		t.Fatalf("grant calls = %d, want 1", len(fixture.tokens.grantCalls))
	}
	grant := fixture.tokens.grantCalls[0]
	if grant.GrantType != "authorization_code" || grant.Code != "the-code" {
		t.Fatalf("grant = %+v", grant)
	}
	if grant.RedirectURI != "https://panel.example.com/api/v1/providers/identity-provider/oauth/callback" {
		t.Fatalf("redirect = %q, want the staged value", grant.RedirectURI)
	}
	if grant.ClientID != "client-identity-provider" {
		t.Fatalf("client id = %q", grant.ClientID)
	}
	if len(grant.CodeVerifier) < 43 {
		t.Fatalf("PKCE verifier missing: %d chars", len(grant.CodeVerifier))
	}
}
