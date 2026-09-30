// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_callback_redirect_test.go
// @for       The staged redirect a callback must spend, and the refusals around it.
// @uses      context, testing, internal/domain.
// @reason    These two cases read the grant the exchange actually sent — redirect, client id, PKCE
//
//	verifier — rather than the account it produced, which is what the callback table next
//	door already pins. `startFlow` lives here because they are the tests that need a real
//	Start instead of a hand-crafted payload, and the table shares it from the same package.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package service

import (
	"context"
	"testing"
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
