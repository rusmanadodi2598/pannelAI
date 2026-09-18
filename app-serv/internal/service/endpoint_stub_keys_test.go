// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_stub_keys_test.go
// @for       The key-scoped half of the in-memory EndpointStore double: key
//
//	append and update, sibling reordering, and the OAuth account match.
//
// @uses      context, internal/domain.
// @reason    AGENTS.md §1.1 caps a file at 250 lines and the double had grown past
//
//	it, so the key methods moved here. They are a natural seam: the real
//	repository keeps a key's statements in endpoint_keys.go for the same
//	reason, and a double that mirrors that split stays readable beside the
//	statements it stands in for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func (s *memEndpointStore) AddKey(_ context.Context, endpointID string, key domain.UpstreamKey) error {
	if _, ok := s.byID[endpointID]; !ok {
		return domain.ErrEndpointNotFound
	}
	s.keysByEndpoint[endpointID] = append(s.keysByEndpoint[endpointID], key)
	return nil
}

func (s *memEndpointStore) UpdateKey(_ context.Context, key domain.UpstreamKey) error {
	keys, ok := s.keysByEndpoint[key.EndpointID()]
	if !ok {
		if _, exists := s.byID[key.EndpointID()]; !exists {
			return domain.ErrEndpointNotFound
		}
	}
	found := false
	for i, existing := range keys {
		if existing.ID() == key.ID() {
			keys[i] = key
			found = true
			break
		}
	}
	if !found {
		return domain.NewNotFoundError("upstream key not found")
	}
	s.keysByEndpoint[key.EndpointID()] = keys
	return nil
}

func (s *memEndpointStore) DeleteKey(_ context.Context, endpointID, keyID string) error {
	keys := s.keysByEndpoint[endpointID]
	for i, key := range keys {
		if key.ID() == keyID {
			s.keysByEndpoint[endpointID] = append(keys[:i:i], keys[i+1:]...)
			return nil
		}
	}
	return domain.NewNotFoundError("upstream key not found")
}

func (s *memEndpointStore) RecordKeyHealth(ctx context.Context, key domain.UpstreamKey) error {
	return s.UpdateKey(ctx, key)
}

func (s *memEndpointStore) Reorder(_ context.Context, providerID string, orderedIDs []string) error {
	for i, id := range orderedIDs {
		endpoint, ok := s.byID[id]
		if !ok {
			return domain.ErrEndpointNotFound
		}
		if endpoint.ProviderID() != providerID {
			return domain.NewConflictError("reorder names an endpoint of another provider")
		}
		if err := endpoint.Update(endpoint.Label(), i+1, "", testNow); err != nil {
			return err
		}
		s.store(endpoint)
	}
	return nil
}

func (s *memEndpointStore) IDsByProvider(_ context.Context, providerID string) ([]string, error) {
	ids := make([]string, 0, len(s.byID))
	for _, endpoint := range s.byID {
		if endpoint.ProviderID() == providerID {
			ids = append(ids, endpoint.ID())
		}
	}
	sortByPriority(ids, s.byID)
	return ids, nil
}

// sortByPriority orders ids by the priority the endpoints currently hold, which is
// what the service reads to renumber siblings.
func sortByPriority(ids []string, byID map[string]domain.UpstreamEndpoint) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && byID[ids[j]].Priority() < byID[ids[j-1]].Priority(); j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

func (s *memEndpointStore) FindOAuthEndpoint(_ context.Context, providerID, email, workspaceID string) (string, error) {
	for _, endpoint := range s.byID {
		if endpoint.ProviderID() != providerID || endpoint.AuthType() != domain.UpstreamAuthOAuth {
			continue
		}
		account := endpoint.Account()
		if (email != "" && account.Email == email) || (workspaceID != "" && account.WorkspaceID == workspaceID) {
			return endpoint.ID(), nil
		}
	}
	return "", domain.ErrEndpointNotFound
}
