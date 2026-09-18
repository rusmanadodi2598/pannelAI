// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_stub_test.go
// @for       The in-memory EndpointStore double the endpoint service's tests are
//
//	built on.
//
// @uses      context, time, internal/domain, internal/repository.
// @reason    AGENTS.md §2.1 forbids t.Skip as a way to sidestep a test, so the service
//
//	is exercised without a database by implementing the store interface here.
//	Keys are held in their own slice per endpoint rather than inside the
//	endpoint value, mirroring the two tables the migration declares: a double
//	that stored keys inside the aggregate would accept a write the real
//	repository performs differently, which is exactly the divergence these
//	tests exist to catch.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// testNow is the fixed instant every fixture is built at.
var testNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// memEndpointStore is an in-memory EndpointStore for service tests.
type memEndpointStore struct {
	byID map[string]domain.UpstreamEndpoint
	// keysByEndpoint mirrors the upstream_keys table, which the schema keeps
	// separate from upstream_endpoints.
	keysByEndpoint map[string][]domain.UpstreamKey
	// labelKey mirrors idx_upstream_endpoints_provider_label, so a duplicate
	// account is refused here exactly as PostgreSQL would refuse it.
	labelKey map[string]string
}

func newMemEndpointStore() *memEndpointStore {
	return &memEndpointStore{
		byID:           map[string]domain.UpstreamEndpoint{},
		keysByEndpoint: map[string][]domain.UpstreamKey{},
		labelKey:       map[string]string{},
	}
}

// accountKey is the uniqueness key the database index uses.
func accountKey(providerID, label string) string { return providerID + "\x00" + label }

// withKeys returns a copy of the endpoint carrying its stored keys, which is what
// every read path in the real repository does with one batched key query.
func (s *memEndpointStore) withKeys(endpoint domain.UpstreamEndpoint) domain.UpstreamEndpoint {
	keys := s.keysByEndpoint[endpoint.ID()]
	return domain.RehydrateUpstreamEndpoint(endpoint.ID(), endpoint.ProviderID(),
		endpoint.Label(), endpoint.AuthType(), endpoint.Priority(), endpoint.Status(),
		endpoint.OAuth(), endpoint.Account(), endpoint.TestStatus(),
		endpoint.RateLimitedUntil(), endpoint.LastUsedAt(),
		endpoint.CreatedAt(), endpoint.UpdatedAt(), keys)
}

// store writes an endpoint's own row, leaving its key rows untouched — the same
// separation the repository's Update keeps.
func (s *memEndpointStore) store(endpoint domain.UpstreamEndpoint) {
	s.byID[endpoint.ID()] = endpoint
}

func (s *memEndpointStore) Create(_ context.Context, endpoint domain.UpstreamEndpoint) error {
	key := accountKey(endpoint.ProviderID(), endpoint.Label())
	if _, dup := s.labelKey[key]; dup {
		return domain.ErrEndpointExists
	}
	s.labelKey[key] = endpoint.ID()
	s.store(endpoint)
	s.keysByEndpoint[endpoint.ID()] = endpoint.Keys()
	return nil
}

// CreateBatch mirrors the real all-or-nothing behaviour: every row is checked first,
// and the offending index is attributed, so a test can prove a refused batch wrote
// nothing.
func (s *memEndpointStore) CreateBatch(_ context.Context, endpoints []domain.UpstreamEndpoint) error {
	if len(endpoints) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(endpoints))
	for i, endpoint := range endpoints {
		key := accountKey(endpoint.ProviderID(), endpoint.Label())
		if _, dup := s.labelKey[key]; dup {
			return rowError(i, domain.ErrEndpointExists)
		}
		if _, dup := seen[key]; dup {
			return rowError(i, domain.ErrEndpointExists)
		}
		seen[key] = struct{}{}
	}
	for _, endpoint := range endpoints {
		s.labelKey[accountKey(endpoint.ProviderID(), endpoint.Label())] = endpoint.ID()
		s.store(endpoint)
		s.keysByEndpoint[endpoint.ID()] = endpoint.Keys()
	}
	return nil
}

func (s *memEndpointStore) AddKeys(_ context.Context, endpointID string, keys []domain.UpstreamKey) error {
	if _, ok := s.byID[endpointID]; !ok {
		return domain.ErrEndpointNotFound
	}
	s.keysByEndpoint[endpointID] = append(s.keysByEndpoint[endpointID], keys...)
	return nil
}

func (s *memEndpointStore) List(_ context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	all := make([]domain.UpstreamEndpoint, 0, len(s.byID))
	for _, endpoint := range s.byID {
		if filter.ProviderID != "" && endpoint.ProviderID() != filter.ProviderID {
			continue
		}
		if filter.Status != "" && string(endpoint.Status()) != filter.Status {
			continue
		}
		all = append(all, s.withKeys(endpoint))
	}
	total := int64(len(all))
	start := q.Offset()
	if start >= len(all) {
		return []domain.UpstreamEndpoint{}, total, nil
	}
	end := start + q.PerPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
}

func (s *memEndpointStore) GetByID(_ context.Context, id string) (domain.UpstreamEndpoint, error) {
	endpoint, ok := s.byID[id]
	if !ok {
		return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
	}
	return s.withKeys(endpoint), nil
}

func (s *memEndpointStore) Update(_ context.Context, endpoint domain.UpstreamEndpoint) error {
	if _, ok := s.byID[endpoint.ID()]; !ok {
		return domain.ErrEndpointNotFound
	}
	s.store(endpoint)
	return nil
}

func (s *memEndpointStore) Delete(_ context.Context, id string) error {
	endpoint, ok := s.byID[id]
	if !ok {
		return domain.ErrEndpointNotFound
	}
	delete(s.labelKey, accountKey(endpoint.ProviderID(), endpoint.Label()))
	delete(s.keysByEndpoint, id)
	delete(s.byID, id)
	return nil
}

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
