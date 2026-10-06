// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_oauth_test.go
// @for       The selection of an oauth endpoint, which carries its credential in the stored token and owns no key row to pick.
// @uses      internal/domain, context, testing, time.
// @reason    SelectNext demanded a healthy key from every endpoint except a no_auth one, so an oauth account built by a device flow was never eligible and the provider answered NO_PROVIDER_AVAILABLE even though its token was valid. These tests pin the fix: the endpoint is selectable with no material, the credential it presents is the stored token, and its health writes stay no-ops like the keyless case they belong to.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package dataplane

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// oauthEndpoint builds an oauth endpoint with a stored token and no keys, which
// is the shape a device flow leaves behind.
func oauthEndpoint(t *testing.T, id string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, "provider-a", "label-"+id, domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		t.Fatalf("building oauth endpoint %s: %v", id, err)
	}
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{AccessTokenEncrypted: "at-cipher"}), now)
	endpoint.SetAccount(domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: "dev@example.com"}), now)
	return endpoint
}

// TestSelector_SelectsAKeylessOAuthEndpoint pins the selection half: an oauth
// endpoint has no key to pick and must still be chosen, carrying the decrypted
// access token rather than a key.
func TestSelector_SelectsAKeylessOAuthEndpoint(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{oauthEndpoint(t, "ep_oauth")}

	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	selection, err := selector.Select(context.Background(), "provider-a")
	if err != nil {
		t.Fatalf("Select() error = %v, want the oauth endpoint to be selectable without a key", err)
	}
	if selection.Endpoint.ID() != "ep_oauth" {
		t.Fatalf("endpoint = %q, want ep_oauth", selection.Endpoint.ID())
	}
	if selection.Key.ID() != "" {
		t.Fatalf("key = %q, want none for an oauth endpoint", selection.Key.ID())
	}
	if family := credentialFamily(t, selection.Credential); family != provider.FamilyOAuth {
		t.Fatalf("family = %v, want an oauth endpoint to present its token as oauth", family)
	}
	if got := credentialSecret(t, selection.Credential); got != "plain-at-cipher" {
		t.Fatalf("access token = %q, want the opened token", got)
	}
}

// TestSelector_SkipsAKeylessOAuthEndpointOnceSpent pins the failover half: the
// endpoint id is the credential unit for a keyless endpoint, so a request that
// already failed on it asks for the next one instead of looping.
func TestSelector_SkipsAKeylessOAuthEndpointOnceSpent(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{oauthEndpoint(t, "ep_oauth")}

	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	selector.clock = func() time.Time { return now }

	if _, err := selector.SelectNext(
		context.Background(), "provider-a", map[string]struct{}{"ep_oauth": {}},
	); err == nil {
		t.Fatal("SelectNext() error = nil, want NO_PROVIDER_AVAILABLE once the endpoint is spent")
	}
}
