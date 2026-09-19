// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_fixture_test.go
// @for       The assembled §7.4 OAuth handler fixture and the request helpers
//
//	every OAuth handler test shares.
//
// @uses      context, net/http, net/http/httptest, strings, testing, time,
//
//	internal/domain, internal/registry, internal/service.
//
// @reason    The handler's job is the HTTP contract, so its tests drive a real
//
//	OAuthFlowService over fakes that answer in memory: no registry
//	binary, no PostgreSQL, no Redis, and no token ever leaves the
//	process. This file is separate from oauth_stub_test.go because the
//	fakes and the fixture are two concerns, and together they would
//	pass the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// oauthNow is the instant these fixtures date their rows with.
var oauthNow = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

// oauthFixture is one assembled handler plus the seams a test asserts on.
type oauthFixture struct {
	handler *OAuthHandler
	store   *oauthStubStore
	states  *oauthStubStates
	tokens  *oauthStubTokens
}

// newOAuthFixture builds the handler over the given providers. baseURL is the
// configured public URL, empty when the deployment declares none.
func newOAuthFixture(t *testing.T, baseURL string, providers ...registry.Provider) oauthFixture {
	t.Helper()
	index := &oauthStubIndex{known: map[string]registry.Provider{}}
	for _, provider := range providers {
		index.known[provider.ID] = provider
	}
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	fixture := oauthFixture{
		store:  newOAuthStubStore(),
		states: newOAuthStubStates(),
		tokens: &oauthStubTokens{},
	}
	svc, err := service.NewOAuthFlowService(service.OAuthFlowDeps{
		Index: index, Store: fixture.store, States: fixture.states,
		Tokens: fixture.tokens, Sealer: sealer,
	})
	if err != nil {
		t.Fatalf("NewOAuthFlowService(): %v", err)
	}
	fixture.handler = NewOAuthHandler(svc, baseURL)
	return fixture
}

// oauthProvider is a code-flow provider with a token endpoint, an authorize
// endpoint, a userinfo endpoint, and a one-hour refresh lead.
func oauthProvider(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			ClientID: "client-" + id, AuthorizeURL: "https://auth.example.com/" + id + "/authorize",
			TokenURL:      "https://auth.example.com/" + id + "/token",
			UserInfoURL:   "https://account.example.com/userinfo",
			Scopes:        registry.StringList{"scope:one", "scope:two"},
			CodeChallenge: "S256", RefreshLeadMS: 3_600_000,
		},
	}
}

// doOAuth runs one request against an OAuth handler method with the path value
// the mux would have set and the Accept header the caller sent.
func doOAuth(t *testing.T, method, target, body, providerID, accept string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if providerID != "" {
		req.SetPathValue("provider_id", providerID)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// startOAuthFlow drives a real Start so a callback test consumes a state that
// was staged the way production stages it.
func startOAuthFlow(t *testing.T, fixture oauthFixture, providerID, redirectURI string) string {
	t.Helper()
	body := `{}`
	if redirectURI != "" {
		body = `{"redirect_uri":"` + redirectURI + `"}`
	}
	rr := doOAuth(t, http.MethodPost, "/api/v1/providers/"+providerID+"/oauth/start",
		body, providerID, "", fixture.handler.Start)
	if rr.Code != http.StatusOK {
		t.Fatalf("start = %d (body: %s)", rr.Code, rr.Body.String())
	}
	state, _ := decodeBody(t, rr)["state"].(string)
	if state == "" {
		t.Fatal("start returned no state")
	}
	return state
}

// seedOAuthAccount plants an OAuth account the way a completed flow would have.
func seedOAuthAccount(t *testing.T, fixture oauthFixture, id, providerID, email string, expiry time.Time) {
	t.Helper()
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	access, err := sealer.Seal("stored-access")
	if err != nil {
		t.Fatalf("sealing the access token: %v", err)
	}
	refresh, err := sealer.Seal("stored-refresh")
	if err != nil {
		t.Fatalf("sealing the refresh token: %v", err)
	}
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, email, domain.UpstreamAuthOAuth, 1, oauthNow)
	if err != nil {
		t.Fatalf("building the endpoint: %v", err)
	}
	endpoint.SetOAuth(&domain.OAuthCredential{
		AccessTokenEncrypted: access, RefreshTokenEncrypted: refresh,
		ExpiresAt: &expiry, AccountEmail: email,
	}, oauthNow)
	endpoint.SetAccount(domain.EndpointAccount{Email: email}, oauthNow)
	if err := fixture.store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing the account: %v", err)
	}
}

// mustErrorCode asserts the response carries the §8 envelope with one code.
func mustErrorCode(t *testing.T, rr *httptest.ResponseRecorder, code string) {
	t.Helper()
	body := decodeBody(t, rr)
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response must carry the error envelope: %v", body)
	}
	if got, _ := errObj["code"].(string); got != code {
		t.Fatalf("code = %q, want %q", got, code)
	}
	if msg, _ := errObj["message"].(string); msg == "" {
		t.Fatal("error message must not be empty")
	}
}
