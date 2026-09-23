// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_combo.go
// @for       The model-string forms resolution walks: a combo's members, the
//
//	provider/model reference, and the memo and depth guard that keep a
//	stored graph from costing more than the request deserves.
//
// @uses      internal/domain, context.
// @reason    SPEC-API-001 §7.7 allows a combo entry to be a provider/model
//
//	reference, a combo name, or an alias, and permits exactly one
//	dereference level. That rule — plus the strategy and judge the
//	fusion path executes — is what this file owns, so resolve.go stays
//	about the resolution entry point and its routability gate.
//
//	Two bounds guard the walk, and they are different things. The depth
//	bound stops a cycle: a stored `A→B→A` terminates instead of
//	recursing. The memo stops the work a diamond multiplies: without it
//	a combo whose members are combos re-walks every subtree once per
//	path to it, so a modest fan-out costs lookups exponentially (a
//	request is one repository read per expansion). Both live in
//	resolveState, which is created per resolution and never shared:
//	the Resolver itself is used concurrently by every request, so a
//	cache on it would be a data race and a correctness hazard.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// comboDepthLimit bounds how many combo links one resolution may follow.
// §7.7 allows one dereference level, but the runtime guard counts the links a
// stored chain actually walks rather than trusting the writer: an alias can
// name a combo whose member names another alias, and each hop re-enters the
// resolver with its own one-level allowance. A stored cycle therefore needs a
// hard stop independent of the level rule — the level rule is a write-time
// contract for honest data, the depth limit is the runtime guarantee for data
// that predates or bypassed it.
const comboDepthLimit = 16

// comboExpansionLimit bounds how many combo members one resolution may expand.
//
// The depth bound alone does not bound the work: with the schema's 64 members
// per combo, a graph of combos that all fail resolves as 64^depth expansions.
// The memo removes the repeated subtrees; this limit is the backstop for the
// graph the memo cannot collapse — a wide DAG with distinct paths — and it is
// expressed in expansions rather than depth because expansions are what cost a
// repository read. Measured against the fixture in resolve_combo_budget_test.go:
// a failing binary tree of depth 8 costs 1,277 lookups without a memo and 17
// with one, and a graph built to exceed this limit stops there rather than
// walking it.
const comboExpansionLimit = 256

// aliasHopLimit bounds how many alias links one resolution may follow. §7.6
// refuses an alias whose target is another alias, so an honest set never hops
// twice; a row written directly into the table can still form `x→y→x`, and this
// is what stops it (measured: without the bound the resolver overflows the
// stack rather than answering).
const aliasHopLimit = 8

// resolveState is one resolution's working set: the combo expansions already
// performed, the answers they produced, and the depth of the branch being
// walked.
//
// A nil map is the zero state, which every entry point uses when the caller has
// no state to share. Because it is created per resolution and passed down, the
// Resolver holds no mutable state and stays safe for concurrent use.
type resolveState struct {
	// combos memoizes a combo's answer by name, including the answers that are
	// failures: a subtree that cannot resolve is walked once, not once per path
	// to it.
	combos map[string]comboAnswer
	// expanded counts the combo expansions this resolution has performed, so a
	// graph too wide to memoize still terminates.
	expanded int
}

// comboAnswer is one memoized combo resolution. A failure travels as its error
// so a repeated path reports the same refusal instead of re-walking for it.
type comboAnswer struct {
	resolution Resolution
	err        error
}

// withState returns a state to walk with, creating one when the caller has none.
func (s *resolveState) withState() *resolveState {
	if s != nil {
		return s
	}
	return &resolveState{combos: make(map[string]comboAnswer)}
}

// resolveCombo resolves the combo's leading routable model and carries the
// combo's shape so the caller can execute the strategy it declares.
//
// The first entry that still resolves wins, and only a combo with no routable
// entry at all fails: a reference left behind by an edit must cost its own
// member, not the whole combo, and both strategies already skip members they
// cannot resolve. A fusion combo ignores the leading choice and fans out to
// every reference instead, which is why the strategy travels with the
// resolution rather than being re-read by the caller.
func (r *Resolver) resolveCombo(ctx context.Context, combo domain.Combo, state *resolveState) (Resolution, error) {
	return r.resolveComboWithin(ctx, combo, 0, 0, state.withState())
}

// resolveComboWithin is resolveCombo with the depth a chain has already
// walked, so a nested member can re-enter resolution without losing the guard.
func (r *Resolver) resolveComboWithin(ctx context.Context, combo domain.Combo, depth, aliasHops int, state *resolveState) (Resolution, error) {
	if combo.ModelCount() == 0 {
		return Resolution{}, dataPlaneError(CodeModelNotFound, "combo "+combo.Name()+" has no models")
	}
	if depth >= comboDepthLimit {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"combo "+combo.Name()+" is part of a chain longer than the resolver follows")
	}
	if memo, ok := state.combos[combo.Name()]; ok {
		return memo.resolution, memo.err
	}
	if state.expanded >= comboExpansionLimit {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"combo "+combo.Name()+" is part of a graph with more members than the resolver expands")
	}
	state.expanded++

	var lastErr error
	for _, ref := range combo.Refs() {
		resolved, err := r.resolveMember(ctx, ref, depth, aliasHops, state)
		if err != nil {
			lastErr = err
			continue
		}
		// The answered identity is the combo the client addressed, never an
		// inner one: the response identity, the rotation key, and the usage row
		// are all keyed by that name, and attributing the request to the inner
		// combo would spend the wrong rotation and record the wrong row.
		resolved.Combo = combo
		state.remember(combo.Name(), resolved, nil)
		return resolved, nil
	}
	if lastErr == nil {
		lastErr = dataPlaneError(CodeModelNotFound, "combo "+combo.Name()+" has no routable member")
	}
	state.remember(combo.Name(), Resolution{}, lastErr)
	return Resolution{}, lastErr
}

// remember stores one combo's answer. A success is stored without its combo
// identity: the identity belongs to whoever addressed the combo, and a memoized
// answer reused from a nested path must not claim the outer combo's name.
func (s *resolveState) remember(name string, resolution Resolution, err error) {
	if s == nil {
		return
	}
	resolution.Combo = domain.Combo{}
	s.combos[name] = comboAnswer{resolution: resolution, err: err}
}
