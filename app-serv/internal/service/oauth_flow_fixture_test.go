// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_fixture_test.go
// @for       The registry providers, fakes, and constructor the OAuth flow
//
//	tests share.
//
// @uses      testing, time, encoding/json, internal/domain, internal/registry,
//
//	internal/repository.
//
// @reason    The flow's rules must be provable without a registry binary, an
//
//	upstream, or Redis: the fakes stage and serve states and token
//	grants in memory, and the providers here mirror the four registry
//	shapes that matter (code flow with PKCE, code flow with userinfo,
//	device flow, connector-required flow) without naming real vendors.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// oauthTestKey seals test credentials; every test in this file shares it so a
// sealed value written by one test could be opened by another if it needed to.
var oauthTestKey = []byte("0123456789abcdef0123456789abcdef")

// fakeStateStore stages and takes OAuth states in memory, recording the TTL a
// caller chose so the ten-minute rule is provable.
type fakeStateStore struct {
	staged map[string]fakeStagedState
	ttl    time.Duration
}

type fakeStagedState struct {
	payload []byte
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{staged: map[string]fakeStagedState{}}
}

func (f *fakeStateStore) Stage(_ context.Context, state string, payload []byte, ttl time.Duration) error {
	if _, dup := f.staged[state]; dup {
		return repository.ErrStateAlreadyStaged
	}
	f.staged[state] = fakeStagedState{payload: payload}
	f.ttl = ttl
	return nil
}

func (f *fakeStateStore) Take(_ context.Context, state string) ([]byte, bool, error) {
	staged, ok := f.staged[state]
	if !ok {
		return nil, false, nil
	}
	delete(f.staged, state)
	return staged.payload, true, nil
}

// fakeTokenClient serves scripted token grants and identities, capturing the
// grants it received so a test can assert the wire values.
type fakeTokenClient struct {
	grantCalls []TokenGrant
	grantFn    func(TokenGrant) (TokenResponse, error)
	infoCalls  int
	infoFn     func() (OAuthIdentity, error)
}

func (f *fakeTokenClient) Grant(_ context.Context, _ string, _ string, grant TokenGrant) (TokenResponse, error) {
	f.grantCalls = append(f.grantCalls, grant)
	if f.grantFn != nil {
		return f.grantFn(grant)
	}
	return TokenResponse{AccessToken: "at-issued", RefreshToken: "rt-issued", ExpiresIn: 3600}, nil
}

func (f *fakeTokenClient) UserInfo(context.Context, string, string) (OAuthIdentity, error) {
	f.infoCalls++
	if f.infoFn != nil {
		return f.infoFn()
	}
	return OAuthIdentity{}, nil
}

// oauthFlowFixture is one assembled flow service plus its seams.
type oauthFlowFixture struct {
	service *OAuthFlowService
	store   *memEndpointStore
	states  *fakeStateStore
	tokens  *fakeTokenClient
	sealer  *domain.Sealer
}

func newOAuthFlowFixture(t *testing.T, providers ...registry.Provider) oauthFlowFixture {
	t.Helper()
	index := &fakeIndex{known: map[string]registry.Provider{}}
	for _, provider := range providers {
		index.known[provider.ID] = provider
	}
	sealer, err := domain.NewSealer(oauthTestKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	fixture := oauthFlowFixture{
		store:  newMemEndpointStore(),
		states: newFakeStateStore(),
		tokens: &fakeTokenClient{},
		sealer: sealer,
	}
	svc, err := NewOAuthFlowService(OAuthFlowDeps{
		Index: index, Store: fixture.store, States: fixture.states, Tokens: fixture.tokens, Sealer: sealer,
	})
	if err != nil {
		t.Fatalf("NewOAuthFlowService: %v", err)
	}
	svc.clock = func() time.Time { return testNow }
	fixture.service = svc
	return fixture
}

// providerWithCodeFlow mirrors a registry entry like a PKCE-protected vendor:
// authorize + token URLs, scopes, S256 challenge, JSON refresh bodies.
func providerWithCodeFlow(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			ClientID: "client-" + id, AuthorizeURL: "https://auth.example.com/" + id + "/authorize",
			TokenURL:      "https://auth.example.com/" + id + "/token",
			Scopes:        registry.StringList{"scope:one", "scope:two"},
			CodeChallenge: "S256", RefreshLeadMS: 3_600_000,
			Refresh: &registry.OAuthRefresh{Encoding: "json"},
		},
	}
}

// providerWithIdentity adds a userinfo endpoint, which is what turns a token
// set into an account identity.
func providerWithIdentity(id string) registry.Provider {
	provider := providerWithCodeFlow(id)
	provider.OAuth.UserInfoURL = "https://account.example.com/userinfo"
	return provider
}

// providerWithDeviceFlow has a device authorization endpoint and no browser
// authorize URL, so the generic code flow cannot serve it.
func providerWithDeviceFlow(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{DeviceAuthURL: "https://oidc.example.com/device", TokenURL: "https://oidc.example.com/token"},
	}
}

// providerNeedingConnector declares a poll-style state URL: its exchange is
// connector territory, not the shared client's.
func providerNeedingConnector(id string) registry.Provider {
	return registry.Provider{
		ID: id, Transport: registry.Transport{Format: "openai"},
		OAuth: &registry.OAuth{
			ClientID: "client-" + id, AuthorizeURL: "https://auth.example.com/authorize",
			StateURL: "https://relay.example.com/state", TokenURL: "https://auth.example.com/token",
		},
	}
}

// decodeStagedState reads back what Start staged, so tests can assert on the
// private context a callback will consume (verifier, redirect, provider).
func decodeStagedState(t *testing.T, raw []byte) oauthStatePayload {
	t.Helper()
	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decoding staged state: %v", err)
	}
	return payload
}

// seedOAuthEndpoint plants an OAuth account directly in the store, the way a
// previous flow or import would have left it.
func seedOAuthEndpoint(t *testing.T, fixture oauthFlowFixture, id, providerID, label, email string, expiry time.Time) domain.UpstreamEndpoint {
	t.Helper()
	access, err := fixture.sealer.Seal("old-access")
	if err != nil {
		t.Fatalf("seeding access token: %v", err)
	}
	refresh, err := fixture.sealer.Seal("old-refresh")
	if err != nil {
		t.Fatalf("seeding refresh token: %v", err)
	}
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, label, domain.UpstreamAuthOAuth, 1, testNow)
	if err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	endpoint.SetOAuth(&domain.OAuthCredential{
		AccessTokenEncrypted: access, RefreshTokenEncrypted: refresh,
		ExpiresAt: &expiry, AccountEmail: email,
	}, testNow)
	endpoint.SetAccount(domain.EndpointAccount{Email: email}, testNow)
	if err := fixture.store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("storing seeded endpoint: %v", err)
	}
	return endpoint
}
