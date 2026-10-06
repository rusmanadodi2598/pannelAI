// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_stub_test.go
// @for       The in-memory seams the §7.4 OAuth handler tests are built from.
// @uses      context, sync, testing, time, internal/domain, internal/registry, internal/repository, internal/service.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
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

// oauthStubStore is the aggregate store the flow reads and writes, in memory.
type oauthStubStore struct {
	mu   sync.Mutex
	byID map[string]domain.UpstreamEndpoint
}

func newOAuthStubStore() *oauthStubStore {
	return &oauthStubStore{byID: map[string]domain.UpstreamEndpoint{}}
}

func (s *oauthStubStore) Create(_ context.Context, endpoint domain.UpstreamEndpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.byID {
		if existing.ProviderID() == endpoint.ProviderID() && existing.Label() == endpoint.Label() {
			return domain.ErrEndpointExists
		}
	}
	s.byID[endpoint.ID()] = endpoint
	return nil
}

func (s *oauthStubStore) List(_ context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]domain.UpstreamEndpoint, 0, len(s.byID))
	for _, endpoint := range s.byID {
		if filter.ProviderID != "" && endpoint.ProviderID() != filter.ProviderID {
			continue
		}
		matched = append(matched, endpoint)
	}
	total := int64(len(matched))
	start := q.Offset()
	if start >= len(matched) {
		return []domain.UpstreamEndpoint{}, total, nil
	}
	end := start + q.PerPage
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func (s *oauthStubStore) GetByID(_ context.Context, id string) (domain.UpstreamEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoint, ok := s.byID[id]
	if !ok {
		return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
	}
	return endpoint, nil
}

func (s *oauthStubStore) Update(_ context.Context, endpoint domain.UpstreamEndpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[endpoint.ID()]; !ok {
		return domain.ErrEndpointNotFound
	}
	s.byID[endpoint.ID()] = endpoint
	return nil
}

func (s *oauthStubStore) UpdateIfUnchanged(_ context.Context, endpoint domain.UpstreamEndpoint, loadedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.byID[endpoint.ID()]
	if !ok {
		return domain.ErrEndpointNotFound
	}
	if !stored.UpdatedAt().Equal(loadedAt) {
		return domain.NewConflictError("the endpoint changed during this refresh")
	}
	s.byID[endpoint.ID()] = endpoint
	return nil
}

func (s *oauthStubStore) FindOAuthEndpoint(_ context.Context, providerID, email, workspaceID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, endpoint := range s.byID {
		if endpoint.ProviderID() != providerID {
			continue
		}
		credential := endpoint.OAuth()
		if credential == nil {
			continue
		}
		if email != "" && credential.AccountEmail().String() == email {
			return endpoint.ID(), nil
		}
		if workspaceID != "" && credential.AccountID() == workspaceID {
			return endpoint.ID(), nil
		}
	}
	return "", domain.ErrEndpointNotFound
}

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
