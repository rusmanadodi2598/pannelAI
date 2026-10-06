// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_usage_stub_test.go
// @for       The doubles the published-quota read is tested against: a lookup, a recording fetcher, and the accounts it asks with.
// @uses      context, internal/domain, internal/registry, internal/service/quotafetch, testing.
// @reason    The read is one seam away from the network, so everything that replaces a network or a store lives here and the behaviour file stays short enough to read as a list of decisions.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// publishedLookup answers GetByID with the one endpoint a case declares, so the
// read's endpoint decision is driven without a store.
type publishedLookup struct {
	endpoint domain.UpstreamEndpoint
	err      error
}

func (l publishedLookup) GetByID(context.Context, string) (domain.UpstreamEndpoint, error) {
	return l.endpoint, l.err
}

// recordingFetcher is the published-quota seam: it answers what the case declares
// and keeps what it was asked with, so "refused before calling" is a call count
// rather than a guess.
type recordingFetcher struct {
	result quotafetch.Result
	calls  int
	family string
	creds  quotafetch.Credentials
}

func (f *recordingFetcher) Fetch(_ context.Context, family string, creds quotafetch.Credentials) quotafetch.Result {
	f.calls++
	f.family = family
	f.creds = creds
	return f.result
}

// usageEntry declares a provider that publishes a usage endpoint, with or without
// one that accepts a key.
func usageEntry(id string, acceptsKey bool) registry.Provider {
	return registry.Provider{
		ID:       id,
		Features: registry.Features{Usage: true, UsageAPIKey: acceptsKey},
	}
}

// oauthPublishedEndpoint builds an account that authenticated by flow and holds the
// given access token in its stored, sealed form.
func oauthPublishedEndpoint(t *testing.T, providerID, accessToken string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_pub", providerID, "acct@example.com", domain.UpstreamAuthOAuth, 1, testNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	sealed, err := newTestSealer(t).Seal(accessToken)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted: sealed, AccountEmail: "acct@example.com",
		// The facts a family's usage endpoint names rather than derives. They are on the
		// fixture because the credential builder is expected to hand them over, and a
		// fixture without them cannot show that it did.
		ProjectID: "proj-pub", AccountID: "user-pub",
	}), testNow)
	return endpoint
}

// keyPublishedEndpoint builds a key account holding the given secret.
func keyPublishedEndpoint(t *testing.T, providerID, key string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_pub", providerID, "keyed", domain.UpstreamAuthAPIKey, 1, testNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	endpoint.SetAccount(domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Name: "keyed", Email: "key@example.com"}), testNow)
	sealed, err := newTestSealer(t).Seal(key)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	credential, err := domain.NewUpstreamKey("primary", endpoint.ID(), "", sealed, "…key", 1, testNow)
	if err != nil {
		t.Fatalf("NewUpstreamKey() error = %v", err)
	}
	endpoint.AttachKey(credential)
	return endpoint
}

// newPublishedFixture wires the real service over the three doubles, with the
// provider index carrying only the entry a case declares.
func newPublishedFixture(t *testing.T, endpoint domain.UpstreamEndpoint, lookErr error,
	entry registry.Provider, result quotafetch.Result) (*QuotaService, *recordingFetcher) {
	t.Helper()
	fetcher := &recordingFetcher{result: result}
	quotas, err := NewQuotaService(QuotaServiceDeps{
		Quotas:    &stubCapRepo{caps: map[string]domain.QuotaCap{}},
		Endpoints: publishedLookup{endpoint: endpoint, err: lookErr},
		Providers: &fakeIndex{known: map[string]registry.Provider{entry.ID: entry}},
		Sealer:    newTestSealer(t),
		FetchUsage: func(ctx context.Context, family string, creds quotafetch.Credentials) quotafetch.Result {
			return fetcher.Fetch(ctx, family, creds)
		},
	})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	return quotas, fetcher
}
