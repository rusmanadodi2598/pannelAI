// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/selection_candidates.go
// @for       The credential walk a request makes over one provider's accounts: the first pick, and the next pick after a credential failed.
// @uses      internal/domain, context.
// @reason    SPEC-API-001 §7.7 fixes the failover order as credential-first: a failed credential is followed by the provider's next healthy one before the next model is tried. The walk is policy the whole data plane depends on, so it lives in one place beside the ordering rule it walks, and the exclude set is what keeps one request from retrying a credential it already saw fail.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-24
package dataplane

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SelectNext returns the first usable credential of a provider outside the spent
// set, so a request that already tried one account asks for the next rather than
// failing. The walk follows the provider's rotation policy: fill-first serves
// endpoints in priority order from the first usable one and picks its first healthy
// key by priority, while round-robin advances a shared cursor, keeps one endpoint for
// its sticky limit, and picks the least-recently-used key. A keyless endpoint is
// itself the candidate either way, which covers the credential-free accounts and the
// oauth ones a login flow built: those carry their token on the endpoint rather than
// in a key row. Every round-robin call advances the cursor, so one walk can leave it
// several steps on: rotation is an optimisation, not a correctness input.
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
	var credErr error
	for i := range endpoints {
		endpoint := endpoints[(offset+i)%len(endpoints)]
		selection, ok, err := s.pickFrom(ctx, endpoint, now, spent, policy)
		if err != nil {
			// An unreadable credential is one row's problem, not the provider's: a
			// rotated process key leaves stale ciphertext on a single endpoint while
			// the healthy accounts beside it still answer. Aborting here made one bad
			// row take the whole provider down. The error is kept so the walk can
			// report it if nothing else was usable either.
			credErr = err
			continue
		}
		if ok {
			return selection, nil
		}
	}
	if credErr != nil {
		return Selection{}, credErr
	}
	return Selection{}, domain.NewNoProviderAvailableError("every upstream endpoint for provider " +
		providerID + " is unavailable, has no usable key, or has spent its budget")
}

// SelectEndpoint returns the credential walk's pick inside one named endpoint,
// which is what an account-bound route needs: an async video job lives on the
// account that created it, so its poll must reach that account rather than
// whichever one the provider's rotation would pick next.
//
// The pin reads no rotation cursor: a caller that named an endpoint has already
// chosen, and advancing the cursor would move the unpinned routes on. A pin
// naming an endpoint the provider does not have is a request defect the client
// can fix; one that exists but is parked, out of budget, or has no usable key
// answers the same unavailability the unpinned walk reports.
func (s *Selector) SelectEndpoint(ctx context.Context, providerID, endpointID string) (Selection, error) {
	now := s.clock()
	pinned := strings.TrimSpace(endpointID)
	endpoints, err := s.candidates(ctx, providerID)
	if err != nil {
		return Selection{}, err
	}
	policy := s.rotationPolicy(ctx, providerID)
	found := false
	var credErr error
	for _, endpoint := range endpoints {
		if endpoint.ID() != pinned {
			continue
		}
		found = true
		selection, ok, err := s.pickFrom(ctx, endpoint, now, nil, policy)
		if err != nil {
			credErr = err
			continue
		}
		if ok {
			return selection, nil
		}
	}
	if credErr != nil {
		return Selection{}, credErr
	}
	if !found {
		return Selection{}, dataPlaneError(CodeValidation,
			"endpoint "+pinned+" is not an active endpoint of provider "+providerID)
	}
	return Selection{}, domain.NewNoProviderAvailableError("endpoint " + pinned + " for provider " + providerID +
		" is unavailable, has no usable key, or has spent its budget")
}

// pickFrom tries one endpoint and answers the selection the walk would make, or
// false when the endpoint cannot serve this request. Both entry points above
// differ only in which endpoints they offer it and what they say when none
// answered, so the per-endpoint rule lives here once.
func (s *Selector) pickFrom(
	ctx context.Context,
	endpoint domain.UpstreamEndpoint,
	now time.Time,
	spent map[string]struct{},
	policy domain.RotationPolicy,
) (Selection, bool, error) {
	if !endpoint.Available(now) {
		return Selection{}, false, nil
	}
	if s.overBudget(ctx, endpoint.ID()) {
		return Selection{}, false, nil
	}
	var key domain.UpstreamKey
	if endpoint.AuthType() == domain.UpstreamAuthAPIKey {
		picked, ok := pickKey(endpoint, now, spent, policy)
		if !ok {
			return Selection{}, false, nil
		}
		key = picked
	} else if _, tried := spent[endpoint.ID()]; tried {
		return Selection{}, false, nil
	}
	credential, err := s.credential(endpoint, key)
	if err != nil {
		return Selection{}, false, err
	}
	return Selection{Endpoint: endpoint, Key: key, Credential: credential}, true, nil
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
