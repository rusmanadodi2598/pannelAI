// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_stub_batch_test.go
// @for       The multi-row writes of the in-memory EndpointStore double, and the all-or-nothing rule they share.
// @uses      context, internal/domain.
// @reason    The real repository keeps these in endpoint_batch.go and endpoint_oauth_batch.go, apart from the single-row writes, because a batch refuses or lands as a whole; the double splits the same way so a test can prove nothing was written when one row is refused.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-06
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

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

// ImportOAuthBatch mirrors the real all-or-nothing import: every row is applied
// only after the whole batch has been checked, and the offending index is
// attributed. It exists because the OAuth import both creates and updates, which
// CreateBatch cannot express.
func (s *memEndpointStore) ImportOAuthBatch(_ context.Context, endpoints []domain.UpstreamEndpoint, existing []bool) error {
	if len(endpoints) == 0 {
		return nil
	}
	if len(existing) != len(endpoints) {
		return domain.NewInternalError("oauth import batch: existing flags do not match the endpoint count")
	}
	seen := make(map[string]struct{}, len(endpoints))
	for i, endpoint := range endpoints {
		if existing[i] {
			if _, ok := s.byID[endpoint.ID()]; !ok {
				return rowError(i, domain.ErrEndpointNotFound)
			}
			continue
		}
		key := accountKey(endpoint.ProviderID(), endpoint.Label())
		if _, dup := s.labelKey[key]; dup {
			return rowError(i, domain.ErrEndpointExists)
		}
		if _, dup := seen[key]; dup {
			return rowError(i, domain.ErrEndpointExists)
		}
		seen[key] = struct{}{}
	}
	for i, endpoint := range endpoints {
		if existing[i] {
			s.store(endpoint)
			continue
		}
		s.labelKey[accountKey(endpoint.ProviderID(), endpoint.Label())] = endpoint.ID()
		s.store(endpoint)
		s.keysByEndpoint[endpoint.ID()] = endpoint.Keys()
	}
	return nil
}
