// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/selection_plan.go
// @for       One credential walk's snapshot: the provider's candidates, its rotation policy, the cursors both read, and the picks the walk takes from them.
// @uses      internal/domain, internal/provider, context, time.
// @reason    The walk used to reload the provider's endpoints and settings on every single attempt, so a twenty-credential failover asked PostgreSQL twenty times, and it re-rolled the rotation cursor per attempt too, which broke the sticky rule mid-walk and re-offered accounts the request had already passed (SPEC-API-001 §7.5 says the cursor advances once per request, not once per attempt). One snapshot per leg makes both statements true and makes the walk a read over data it already holds.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
)

// SelectionPlan is one provider leg's view of what routing may spend: a snapshot rather
// than a live query, so a long failover cannot redefine "healthy" halfway through and
// offer one account twice. The walk that fails over from a key to its sibling sees the
// accounts, windows and budget the leg started with.
type SelectionPlan struct {
	selector   *Selector
	providerID string
	endpoints  []domain.UpstreamEndpoint
	policy     domain.RotationPolicy
	now        time.Time
	// endpointOffset is where the account walk starts, taken once for the leg.
	endpointOffset int
	// keyOffsets holds the cursor value one endpoint was handed, so a second pick in
	// the same account continues from there instead of rolling the counter again.
	keyOffsets map[string]int
	// budgets caches the §7.12 gate per endpoint, because a walk that reaches the
	// same account again must not ask the quota table twice.
	budgets map[string]bool
}

// BeginPlan loads what one credential walk needs, once. An unreadable candidate list is
// the provider's failure, not the plan's: the caller reports it as the walk used to.
func (s *Selector) BeginPlan(ctx context.Context, providerID string) (*SelectionPlan, error) {
	endpoints, err := s.candidates(ctx, providerID)
	if err != nil {
		return nil, err
	}
	policy := s.rotationPolicy(ctx, providerID)
	plan := &SelectionPlan{
		selector:   s,
		providerID: providerID,
		endpoints:  endpoints,
		policy:     policy,
		now:        s.clock(),
		keyOffsets: map[string]int{},
		budgets:    map[string]bool{},
	}
	if policy.UsesRotation() {
		plan.endpointOffset = s.offset(ctx, providerID, len(endpoints), policy.StickyLimit)
	}
	return plan, nil
}

// Next returns the first credential this leg has not spent, or false when the snapshot
// has nothing left. It reloads nothing: the accounts, their keys, the rotation position
// and the budget answers are already held, and the only call it can still make is the
// key cursor, once per account.
//
// A credential error is one row's problem, not the provider's: a rotated process key
// leaves stale ciphertext on one account while its neighbours still answer, so the walk
// logs it and serves a later credential. It reaches the caller only when nothing answered.
func (p *SelectionPlan) Next(ctx context.Context, spent map[string]struct{}) (Selection, bool, error) {
	if len(p.endpoints) == 0 {
		return Selection{}, false, nil
	}
	var credentialErr error
	for step := range p.endpoints {
		endpoint := p.endpoints[(p.endpointOffset+step)%len(p.endpoints)]
		selection, ok, err := p.pickFrom(ctx, endpoint, spent)
		if err != nil {
			credentialErr = err
			continue
		}
		if ok {
			if credentialErr != nil {
				logx.Degraded(p.selector.logger, "one account's credential was unreadable, a later one served", credentialErr,
					"provider_id", p.providerID)
			}
			p.selector.markSelected(ctx, selection)
			return selection, true, nil
		}
	}
	return Selection{}, false, credentialErr
}

// pickFrom answers one account for this leg: whether it may serve, which of its
// keys the walk spends, and the credential that key stands for.
func (p *SelectionPlan) pickFrom(
	ctx context.Context,
	endpoint domain.UpstreamEndpoint,
	spent map[string]struct{},
) (Selection, bool, error) {
	if !endpoint.Available(p.now) {
		return Selection{}, false, nil
	}
	if p.overBudgetCached(ctx, endpoint.ID()) {
		return Selection{}, false, nil
	}
	var key domain.UpstreamKey
	if endpoint.AuthType().UsesStoredKey() {
		picked, ok := p.chooseKey(ctx, endpoint, spent)
		if !ok {
			return Selection{}, false, nil
		}
		key = picked
	} else if _, tried := spent[endpoint.ID()]; tried {
		return Selection{}, false, nil
	}
	credential, err := p.selector.credential(endpoint, key)
	if err != nil {
		return Selection{}, false, err
	}
	return Selection{Endpoint: endpoint, Key: key, Credential: credential}, true, nil
}

// chooseKey applies the policy inside one account: round-robin asks the counter for a
// start position, fill-first takes the first healthy key and reads no counter, so it
// leaves no rotation state behind. The read is memoized per account, because a second
// pick inside one account continues the same walk rather than starting another.
func (p *SelectionPlan) chooseKey(
	ctx context.Context,
	endpoint domain.UpstreamEndpoint,
	spent map[string]struct{},
) (domain.UpstreamKey, bool) {
	if !p.policy.UsesRotation() {
		return endpoint.FirstKeySkipping(p.now, spent)
	}
	offset, read := p.keyOffsets[endpoint.ID()]
	if !read {
		offset = p.readKeyCursor(ctx, endpoint.ID())
		p.keyOffsets[endpoint.ID()] = offset
	}
	if offset < 0 {
		// No cursor: the stamp still advances on every choice, so the degraded path
		// spreads a burst within one round instead of piling on one LRU winner.
		return endpoint.NextKeySkipping(p.now, spent)
	}
	return endpoint.KeyAtOffset(p.now, spent, offset)
}

// readKeyCursor asks one account for its rotation position. A missing seam or a
// failed read answers -1 rather than failing the request: rotation is an
// optimisation, and the stamped-LRU fallback still spreads a burst without it.
func (p *SelectionPlan) readKeyCursor(ctx context.Context, endpointID string) int {
	cursor := p.selector.keyCursor
	if cursor == nil {
		return -1
	}
	offset, err := cursor.Next(ctx, endpointID)
	if err != nil {
		logx.Degraded(p.selector.logger, "key cursor unavailable, selection fell back to the selection stamp", err,
			"endpoint_id", endpointID)
		return -1
	}
	return offset
}

// overBudgetCached asks the §7.12 gate once per account for this leg. The failure
// direction is the selector's: a quota read that breaks is "not over budget",
// because a control-plane outage must not lock every provider out of the data plane.
func (p *SelectionPlan) overBudgetCached(ctx context.Context, endpointID string) bool {
	if p.selector.gates == nil || endpointID == "" {
		return false
	}
	if exhausted, seen := p.budgets[endpointID]; seen {
		return exhausted
	}
	exhausted := p.selector.overBudget(ctx, endpointID)
	p.budgets[endpointID] = exhausted
	return exhausted
}
