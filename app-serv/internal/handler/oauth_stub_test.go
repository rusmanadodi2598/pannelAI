// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_stub_test.go
// @for       The in-memory seams the §7.4 OAuth handler tests are built from.
// @uses      context, sync, testing, time, internal/registry, internal/service.
// @reason    The handler's job is the HTTP contract, so its tests drive a real OAuthFlowService over fakes that answer in memory: no registry binary, no PostgreSQL, no Redis, and no token ever leaves the process. The fakes mirror the four registry shapes that matter (code flow with PKCE, code flow with userinfo, device flow, connector-required flow) without naming a vendor.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-19
package handler

import (
	"context"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
)

// oauthTestKey seals the credentials these tests write.
var oauthTestKey = []byte("0123456789abcdef0123456789abcdef")

// oauthStubIndex is a ProviderIndex over a fixed provider map.
type oauthStubIndex struct {
	known map[string]registry.Provider
}

func (s *oauthStubIndex) Provider(name string) (registry.Provider, bool) {
	provider, ok := s.known[name]
	return provider, ok
}

func (s *oauthStubIndex) All() []registry.Provider {
	all := make([]registry.Provider, 0, len(s.known))
	for _, provider := range s.known {
		all = append(all, provider)
	}
	return all
}

func (s *oauthStubIndex) Categories() []string { return nil }

// oauthStubStates stages states in memory with the single-use rule.
type oauthStubStates struct {
	mu     sync.Mutex
	staged map[string][]byte
}

func newOAuthStubStates() *oauthStubStates {
	return &oauthStubStates{staged: map[string][]byte{}}
}

func (s *oauthStubStates) Stage(_ context.Context, state string, payload []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.staged[state]; dup {
		return repository.ErrStateAlreadyStaged
	}
	s.staged[state] = payload
	return nil
}

func (s *oauthStubStates) Take(_ context.Context, state string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, ok := s.staged[state]
	if !ok {
		return nil, false, nil
	}
	delete(s.staged, state)
	return payload, true, nil
}

// Peek reads a staged state without consuming it, the way a polled flow must.
func (s *oauthStubStates) Peek(_ context.Context, state string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, ok := s.staged[state]
	if !ok {
		return nil, false, nil
	}
	return payload, true, nil
}

// oauthStubTokens serves scripted grants and identities.
type oauthStubTokens struct {
	grantFn       func(oauthhttp.TokenGrant) (oauthhttp.TokenResponse, error)
	infoFn        func() (oauthhttp.OAuthIdentity, error)
	devicePollFn  func(nonce, verifier string) (oauthhttp.DeviceTokenResponse, bool, error)
	devicePollArg []string
}

// DevicePoll answers one device poll from the script, recording the round it was
// asked about so a test can prove the handler passed the code through.
func (s *oauthStubTokens) DevicePoll(_ context.Context, _, nonce, verifier string) (oauthhttp.DeviceTokenResponse, bool, error) {
	s.devicePollArg = []string{nonce, verifier}
	if s.devicePollFn != nil {
		return s.devicePollFn(nonce, verifier)
	}
	return oauthhttp.DeviceTokenResponse{}, true, nil
}

func (s *oauthStubTokens) Grant(_ context.Context, _ string, _ string, grant oauthhttp.TokenGrant) (oauthhttp.TokenResponse, error) {
	if s.grantFn != nil {
		return s.grantFn(grant)
	}
	return oauthhttp.TokenResponse{AccessToken: "at-issued", RefreshToken: "rt-issued", ExpiresIn: 3600}, nil
}

func (s *oauthStubTokens) UserInfo(context.Context, string, string) (oauthhttp.OAuthIdentity, error) {
	if s.infoFn != nil {
		return s.infoFn()
	}
	return oauthhttp.OAuthIdentity{}, nil
}
