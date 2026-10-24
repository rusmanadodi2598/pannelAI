// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/selection_candidates.go
// @for       The two entry points a credential walk offers: the provider's next unused credential, and one named account's credential.
// @uses      internal/domain, context, strings.
// @reason    SPEC-API-001 §7.7 fixes the failover order as credential-first: a failed credential is followed by the provider's next healthy one before the next model is tried. Both questions read the same snapshot through SelectionPlan, so the reload-per-attempt and the cursor-step-per-attempt the walk used to make are impossible by construction, and the exclude set is what keeps one request from retrying a credential it already saw fail.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-24
package dataplane

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SelectNext returns the first usable credential of a provider outside the spent
// set, so a request that already tried one account asks for the next rather than
// failing. Fill-first walks endpoints by priority and picks the first healthy key;
// round-robin takes one cursor position per leg for the accounts and one per account
// for its keys. A keyless endpoint is itself the candidate either way: it carries its
// token on the row rather than in a key. The cursor advances once per leg, not once
// per attempt, so a failover continues the walk it started (SPEC-API-001 §7.5).
func (s *Selector) SelectNext(ctx context.Context, providerID string, spent map[string]struct{}) (Selection, error) {
	plan, err := s.BeginPlan(ctx, providerID)
	if err != nil {
		return Selection{}, err
	}
	if len(plan.endpoints) == 0 {
		return Selection{}, domain.NewNoProviderAvailableError(
			"no upstream endpoint is configured for provider " + providerID)
	}
	selection, ok, err := plan.Next(ctx, spent)
	if err != nil {
		return Selection{}, err
	}
	if !ok {
		return Selection{}, domain.NewNoProviderAvailableError("every upstream endpoint for provider " +
			providerID + " is unavailable, has no usable key, or has spent its budget")
	}
	return selection, nil
}

// SelectEndpoint returns the credential walk's pick inside one named endpoint,
// which is what an account-bound route needs: an async video job lives on the
// account that created it, so its poll must reach that account rather than whichever
// one the provider's rotation would pick next.
//
// The pin advances no account cursor and leaves no stamp: the caller already chose,
// and moving the account walk on would change which account the unpinned routes
// reach next. The key cursor is still read, because the poll spends one credential's
// quota. A pin on an endpoint the provider lacks is a request defect; one that is
// parked or spent reports the same unavailability as the unpinned walk.
func (s *Selector) SelectEndpoint(ctx context.Context, providerID, endpointID string) (Selection, error) {
	now := s.clock()
	endpoints, err := s.candidates(ctx, providerID)
	if err != nil {
		return Selection{}, err
	}
	plan := &SelectionPlan{
		selector: s, providerID: providerID, endpoints: endpoints,
		policy: s.rotationPolicy(ctx, providerID), now: now,
		keyOffsets: map[string]int{}, budgets: map[string]bool{},
	}
	found := false
	var credentialErr error
	for _, endpoint := range endpoints {
		if endpoint.ID() != strings.TrimSpace(endpointID) {
			continue
		}
		found = true
		selection, ok, err := plan.pickFrom(ctx, endpoint, nil)
		if err != nil {
			credentialErr = err
			continue
		}
		if ok {
			return selection, nil
		}
	}
	if credentialErr != nil {
		return Selection{}, credentialErr
	}
	if !found {
		return Selection{}, dataPlaneError(CodeValidation,
			"endpoint "+strings.TrimSpace(endpointID)+" is not an active endpoint of provider "+providerID)
	}
	return Selection{}, domain.NewNoProviderAvailableError("endpoint " + strings.TrimSpace(endpointID) +
		" for provider " + providerID + " is unavailable, has no usable key, or has spent its budget")
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
