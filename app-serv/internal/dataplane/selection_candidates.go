// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/selection_candidates.go
// @for       The credential walk a request makes over one provider's accounts:
//
//	the first pick, and the next pick after a credential failed.
//
// @uses      internal/domain, context.
// @reason    SPEC-API-001 §7.7 fixes the failover order as credential-first: a
//
//	failed credential is followed by the provider's next healthy one
//	before the next model is tried. The walk is policy the whole data
//	plane depends on, so it lives in one place beside the ordering rule
//	it walks, and the exclude set is what keeps one request from
//	retrying a credential it already saw fail.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SelectNext returns the first usable credential of a provider that is outside
// the spent set, so a request that already tried one account asks for the next
// one rather than failing (SPEC-API-001 §7.7, credential-first failover).
//
// The walk visits the provider's endpoints in rotation order and picks the
// least-recently-used healthy key inside the first usable one; a keyless
// endpoint is itself the candidate. Each call advances the rotation cursor, so
// a request that walks two credentials leaves the cursor two steps on;
// rotation stays an optimisation, not a correctness input.
func (s *Selector) SelectNext(ctx context.Context, providerID string, spent map[string]struct{}) (Selection, error) {
	now := s.clock()
	endpoints, err := s.candidates(ctx, providerID)
	if err != nil {
		return Selection{}, err
	}
	if len(endpoints) == 0 {
		return Selection{}, domain.NewNoProviderAvailableError(
			"no upstream endpoint is configured for provider " + providerID)
	}

	offset := s.offset(ctx, providerID, len(endpoints))
	for i := range endpoints {
		endpoint := endpoints[(offset+i)%len(endpoints)]
		if !endpoint.Available(now) {
			continue
		}
		if s.overBudget(ctx, endpoint.ID()) {
			continue
		}
		var key domain.UpstreamKey
		if endpoint.AuthType() != domain.UpstreamAuthNone {
			picked, ok := endpoint.NextKeySkipping(now, spent)
			if !ok {
				continue
			}
			key = picked
		} else if _, tried := spent[endpoint.ID()]; tried {
			continue
		}
		credential, err := s.credential(endpoint, key)
		if err != nil {
			return Selection{}, err
		}
		return Selection{Endpoint: endpoint, Key: key, Credential: credential}, nil
	}
	return Selection{}, domain.NewNoProviderAvailableError("every upstream endpoint for provider " +
		providerID + " is unavailable, has no usable key, or has spent its budget")
}

// CandidateID names the credential unit a selection stands for: the key for a
// keyed endpoint, the endpoint itself for a keyless one. It is what a request
// excludes after a failed attempt, so the next pick is a different credential.
func CandidateID(selection Selection) string {
	if selection.Key.ID() != "" {
		return selection.Key.ID()
	}
	return selection.Endpoint.ID()
}
