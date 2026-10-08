// Package handler serves the management HTTP API of app-serv.
//
// @file      internal/handler/oauth_stub_store_test.go
// @for       The in-memory endpoint store the OAuth flow tests write through.
// @uses      context, sync, internal/domain, internal/repository.
// @reason    The store is the double that has to answer like the repository does, including the compare-and-swap a rotation conditions on, so it is kept apart from the index, state, and token doubles that only ever return a fixed answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-06
package handler

import (
	"context"
	"sync"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// oauthStubStore is the aggregate store the flow reads and writes, in memory.
type oauthStubStore struct {
	mu   sync.Mutex
	byID map[string]domain.UpstreamEndpoint
	// rejectCompareAndSwap loses the compare-and-swap for the named accounts only, so
	// a test can give a sweep one account to trip over.
	rejectCompareAndSwap map[string]bool
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

func (s *oauthStubStore) UpdateIfUnchanged(_ context.Context, endpoint domain.UpstreamEndpoint, loaded domain.OAuthCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.byID[endpoint.ID()]
	if !ok {
		return domain.ErrEndpointNotFound
	}
	if s.rejectCompareAndSwap[endpoint.ID()] || stored.OAuth() == nil ||
		stored.OAuth().AccessTokenEncrypted() != loaded.AccessTokenEncrypted() ||
		stored.OAuth().RefreshTokenEncrypted() != loaded.RefreshTokenEncrypted() {
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
