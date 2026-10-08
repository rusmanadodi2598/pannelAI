// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_stub_test.go
// @for       The in-memory EndpointStore double the endpoint service's tests are built on.
// @uses      context, time, internal/domain, internal/repository.
// @reason    AGENTS.md §2.1 forbids t.Skip as a way to sidestep a test, so the service is exercised without a database by implementing the store interface here.
//
//	Keys are held in their own slice per endpoint rather than inside the
//	endpoint value, mirroring the two tables the migration declares: a double
//	that stored keys inside the aggregate would accept a write the real
//	repository performs differently, which is exactly the divergence these
//	tests exist to catch.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
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
	// casLoadedCredential records the credential the last UpdateIfUnchanged was
	// conditioned on, and casReject makes that call answer the way PostgreSQL does
	// when the row moved under the caller.
	casLoadedCredential domain.OAuthCredential
	casReject           bool
	// casRejectFor loses the compare-and-swap for the named endpoints only, which is
	// what lets a batch test single out one account.
	casRejectFor map[string]bool
	// healthWrites counts the key-health writes a caller made, so a test can pin
	// that an outcome which must not touch the circuit really wrote nothing.
	healthWrites int
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
//
// The parity fields are carried through explicitly, because
// omitting them is the drift this double exists to avoid: the tests would keep
// passing while every routing order, default model, proxy binding, use count, and
// last error vanished between a write and the read that follows it.
func (s *memEndpointStore) withKeys(endpoint domain.UpstreamEndpoint) domain.UpstreamEndpoint {
	keys := s.keysByEndpoint[endpoint.ID()]
	code, message, at := endpoint.LastError()
	return domain.RehydrateUpstreamEndpoint(endpoint.ID(), endpoint.ProviderID(),
		endpoint.Label(), endpoint.AuthType(), endpoint.Priority(), endpoint.Status(),
		endpoint.OAuth(), endpoint.Account(), endpoint.TestStatus(),
		endpoint.RateLimitedUntil(), endpoint.LastUsedAt(),
		endpoint.CreatedAt(), endpoint.UpdatedAt(), keys,
		domain.EndpointParity{
			GlobalPriority: endpoint.GlobalPriority(), DefaultModel: endpoint.DefaultModel(),
			ConsecutiveUseCount: endpoint.ConsecutiveUseCount(), LastErrorCode: code,
			LastErrorMessage: message, LastErrorAt: at, ProxyPoolID: endpoint.ProxyPoolID(),
		})
}

// store writes an endpoint's own row, leaving its key rows untouched, the same
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

func (s *memEndpointStore) UpdateIfUnchanged(_ context.Context, endpoint domain.UpstreamEndpoint, loaded domain.OAuthCredential) error {
	stored, ok := s.byID[endpoint.ID()]
	if !ok {
		return domain.ErrEndpointNotFound
	}
	s.casLoadedCredential = loaded
	if s.casReject || s.casRejectFor[endpoint.ID()] || !sameSealedCredential(stored.OAuth(), loaded) {
		return domain.NewConflictError("the endpoint changed during this refresh")
	}
	s.store(endpoint)
	return nil
}

// sameSealedCredential answers the way the real predicate does: the stored
// credential still holds the two ciphertexts the caller conditioned its write on.
func sameSealedCredential(stored *domain.OAuthCredential, loaded domain.OAuthCredential) bool {
	return stored != nil &&
		stored.AccessTokenEncrypted() == loaded.AccessTokenEncrypted() &&
		stored.RefreshTokenEncrypted() == loaded.RefreshTokenEncrypted()
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
