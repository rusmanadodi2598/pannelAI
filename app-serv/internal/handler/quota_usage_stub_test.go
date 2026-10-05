// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/quota_usage_stub_test.go
// @for       The doubles and the fixture the published-quota route is tested
//
//	against.
//
// @uses      context, encoding/json, internal/domain, internal/registry,
//
//	internal/schema, internal/service, internal/service/quotafetch,
//	net/http/httptest, testing, time.
//
// @reason    The route's behaviour file should read as the list of answers a
//
//	client can get, so the wiring that produces them, a lookup over one
//	endpoint, a sealed credential, a registry entry with named features,
//	lives apart from the assertions.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-28
package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// publishedStubLookup answers GetByID with the one endpoint a case declares.
type publishedStubLookup struct{ endpoint domain.UpstreamEndpoint }

func (l publishedStubLookup) GetByID(context.Context, string) (domain.UpstreamEndpoint, error) {
	if l.endpoint.ID() == "" {
		return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
	}
	return l.endpoint, nil
}

// publishedSealed builds the stored form of one secret, the way the endpoint
// service writes it.
func publishedSealed(t *testing.T, value string) string {
	t.Helper()
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	sealed, err := sealer.Seal(value)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	return sealed
}

// publishedOAuthEndpoint builds a flow-authenticated account holding one token.
func publishedOAuthEndpoint(t *testing.T, providerID, accessToken string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_pub", providerID, "acct@example.com", domain.UpstreamAuthOAuth, 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	endpoint.SetOAuth(domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{
		AccessTokenEncrypted: publishedSealed(t, accessToken), AccountEmail: "acct@example.com",
	}), time.Now().UTC())
	return endpoint
}

// publishedEntry declares one registry provider with the features a case names.
func publishedEntry(id string, usage, usageKey bool) registry.Provider {
	return registry.Provider{
		ID: id, Features: registry.Features{Usage: usage, UsageAPIKey: usageKey},
	}
}

// newPublishedHandler wires the real handler over the real service, with the
// provider read replaced by the answer a case declares. An entry with no id
// declares a registry that knows no provider at all.
func newPublishedHandler(t *testing.T, endpoint domain.UpstreamEndpoint, entry registry.Provider,
	result quotafetch.Result) *QuotaHandler {
	t.Helper()
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	index := &oauthStubIndex{known: map[string]registry.Provider{}}
	if entry.ID != "" {
		index.known[entry.ID] = entry
	}
	quotas, err := service.NewQuotaService(service.QuotaServiceDeps{
		Quotas:    newStubQuotaRepo(),
		Endpoints: publishedStubLookup{endpoint: endpoint},
		Providers: index, Sealer: sealer,
		FetchUsage: func(context.Context, string, quotafetch.Credentials) quotafetch.Result {
			return result
		},
	})
	if err != nil {
		t.Fatalf("NewQuotaService() error = %v", err)
	}
	return NewQuotaHandler(quotas)
}

// decodePublished reads the route's own response shape.
func decodePublished(t *testing.T, rec *httptest.ResponseRecorder) schema.PublishedQuotaUsageResponse {
	t.Helper()
	var body schema.PublishedQuotaUsageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v (%s)", err, rec.Body.String())
	}
	return body
}
