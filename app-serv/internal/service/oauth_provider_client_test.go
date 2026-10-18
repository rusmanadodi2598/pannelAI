// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_provider_client_test.go
// @for       Which OAuth client material a provider's round is performed with.
// @uses      context, net/url, testing, time, internal/registry.
// @reason    The registry carries a provider's client id and never its secret, because the generated document is committed; the secret arrives from typed config. Both halves have to reach the authorize URL, the code exchange, and every later refresh, or a configured client is applied to the login and abandoned for the token that follows it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

func TestOAuthRoundCarriesTheConfiguredClient(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithCodeFlow("gc"))
	fixture.service.providerClients = map[string]OAuthClient{
		"gc": {ClientID: "own-client", ClientSecret: "own-secret"},
	}

	started, err := fixture.service.Start(context.Background(), OAuthStartInput{
		ProviderID: "gc", BaseURL: "https://panel.example.com",
	})
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	parsed, err := url.Parse(started.AuthorizeURL)
	if err != nil {
		t.Fatalf("authorize URL unparseable: %v", err)
	}
	if got := parsed.Query().Get("client_id"); got != "own-client" {
		t.Fatalf("authorize client_id = %q, want the configured client", got)
	}

	if _, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "gc", Code: "the-code", State: started.State,
	}); err != nil {
		t.Fatalf("Callback(): %v", err)
	}
	grant := lastGrant(fixture.tokens)
	if grant.ClientID != "own-client" {
		t.Fatalf("exchange client_id = %q, want the configured client", grant.ClientID)
	}
	if grant.ClientSecret != "own-secret" {
		t.Fatalf("exchange client_secret = %q, want the configured secret", grant.ClientSecret)
	}
}

// TestOAuthRoundFallsBackToTheRegistryClient holds the other half of the rule:
// an unconfigured provider is not broken by the override existing, and an empty
// secret stays empty rather than becoming a blank that the grant then sends.
func TestOAuthRoundFallsBackToTheRegistryClient(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithCodeFlow("gc"))

	started, err := fixture.service.Start(context.Background(), OAuthStartInput{
		ProviderID: "gc", BaseURL: "https://panel.example.com",
	})
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	parsed, _ := url.Parse(started.AuthorizeURL)
	if got := parsed.Query().Get("client_id"); got != "client-gc" {
		t.Fatalf("authorize client_id = %q, want the registry entry's client", got)
	}

	if _, err := fixture.service.Callback(context.Background(), OAuthCallbackInput{
		ProviderID: "gc", Code: "the-code", State: started.State,
	}); err != nil {
		t.Fatalf("Callback(): %v", err)
	}
	grant := lastGrant(fixture.tokens)
	if grant.ClientID != "client-gc" {
		t.Fatalf("exchange client_id = %q, want the registry entry's client", grant.ClientID)
	}
	if grant.ClientSecret != "" {
		t.Fatalf("exchange client_secret = %q, want none for an unconfigured provider", grant.ClientSecret)
	}
}

// TestOAuthRefreshCarriesTheConfiguredClient proves the third path: a token the
// vendor rotates days later is exchanged with the same client that opened the
// round, because a refresh under an id the deployment does not hold fails against
// a secret it alone knows.
func TestOAuthRefreshCarriesTheConfiguredClient(t *testing.T) {
	fixture := newOAuthFlowFixture(t, providerWithCodeFlow("gc"))
	fixture.service.providerClients = map[string]OAuthClient{"gc": {ClientSecret: "own-secret"}}
	seedOAuthEndpoint(t, fixture, "ep_gc", "gc", "Account 1", "someone@example.com", testNow.Add(time.Hour))

	if _, err := fixture.service.Refresh(context.Background(), "gc", "ep_gc"); err != nil {
		t.Fatalf("Refresh(): %v", err)
	}
	grant := lastGrant(fixture.tokens)
	if grant.ClientSecret != "own-secret" {
		t.Fatalf("refresh client_secret = %q, want the configured secret", grant.ClientSecret)
	}
	if grant.ClientID != "client-gc" {
		t.Fatalf("refresh client_id = %q, want the registry entry's client when no id is configured", grant.ClientID)
	}
}

// TestOAuthRoundRefusesAnUnconfiguredClient pins the half the registry no longer
// supplies. A flow whose client belongs to the deployment is refused when the
// deployment states nothing, and it is refused before any state is staged, so the
// panel gets one English sentence about its own environment instead of a browser
// redirect that dies at the provider.
func TestOAuthRoundRefusesAnUnconfiguredClient(t *testing.T) {
	entry := providerWithCodeFlow("gc")
	entry.OAuth.ClientID = ""
	fixture := newOAuthFlowFixture(t, entry)

	_, err := fixture.service.Start(context.Background(), OAuthStartInput{
		ProviderID: "gc", BaseURL: "https://panel.example.com",
	})
	if err == nil {
		t.Fatal("Start() error = nil, want the round refused for want of a client")
	}
	if !strings.Contains(err.Error(), "not configured on this deployment") {
		t.Fatalf("Start() error = %v, want the reason an operator can act on", err)
	}
	if len(fixture.states.staged) != 0 {
		t.Fatalf("staged states = %d, want a refused round to leave nothing behind", len(fixture.states.staged))
	}
}

// TestOAuthRefreshRefusesWhenTheClientDisappearsLater holds the path a refusal at
// Start cannot cover: the credential was connected while the deployment stated its
// client, and the env line is gone by the time the token needs rotating. The
// worker is told the same sentence rather than exchanging under no client.
func TestOAuthRefreshRefusesWhenTheClientDisappearsLater(t *testing.T) {
	entry := providerWithCodeFlow("gc")
	entry.OAuth.ClientID = ""
	fixture := newOAuthFlowFixture(t, entry)
	seedOAuthEndpoint(t, fixture, "ep_gc", "gc", "Account 1", "someone@example.com", testNow.Add(time.Hour))

	if _, err := fixture.service.Refresh(context.Background(), "gc", "ep_gc"); err == nil {
		t.Fatal("Refresh() error = nil, want the round refused for want of a client")
	} else if !strings.Contains(err.Error(), "not configured on this deployment") {
		t.Fatalf("Refresh() error = %v, want the reason an operator can act on", err)
	}
	if calls := len(fixture.tokens.grantCalls); calls != 0 {
		t.Fatalf("grant calls = %d, want a refused refresh to reach no provider", calls)
	}
}

func lastGrant(client *fakeTokenClient) oauthhttp.TokenGrant {
	calls := client.grantCalls
	if len(calls) == 0 {
		return oauthhttp.TokenGrant{}
	}
	return calls[len(calls)-1]
}
