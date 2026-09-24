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
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SelectNext returns the first usable credential of a provider that is outside
// the spent set, so a request that already tried one account asks for the next
// one rather than failing (SPEC-API-001 §7.7, credential-first failover).
//
// The walk follows the provider's rotation policy (§7.14): fill-first serves
// the provider's endpoints in priority order and starts every request from the
// first usable one, while round-robin advances the shared cursor and keeps one
// endpoint for the policy's sticky limit. Inside the endpoint the policy picks
// the key: the first healthy one by priority under fill-first, the
// least-recently-used one under round-robin. A keyless endpoint is itself the
// candidate either way. Each round-robin call advances the rotation cursor, so
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

	policy := s.rotationPolicy(ctx, providerID)
	offset := 0
	if policy.UsesRotation() {
		offset = s.offset(ctx, providerID, len(endpoints), policy.StickyLimit)
	}
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
			picked, ok := pickKey(endpoint, now, spent, policy)
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

// pickKey applies the policy's key rule inside one endpoint: the first healthy
// key by priority under fill-first, the least-recently-used one under
// round-robin. Both skip the request's spent set, so a failed credential is
// never retried inside one request.
func pickKey(
	endpoint domain.UpstreamEndpoint,
	now time.Time,
	spent map[string]struct{},
	policy domain.RotationPolicy,
) (domain.UpstreamKey, bool) {
	if policy.UsesRotation() {
		return endpoint.NextKeySkipping(now, spent)
	}
	return endpoint.FirstKeySkipping(now, spent)
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
