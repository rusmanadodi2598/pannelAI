// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_combo.go
// @for       The two model-string forms resolution walks: a combo's first
//
//	member and a provider/model reference.
//
// @uses      internal/domain, context.
// @reason    SPEC-API-001 §7.7 allows a combo entry to be a provider/model
//
//	reference, a combo name, or an alias, and permits exactly one
//	dereference level. That rule — plus the strategy and judge the
//	fusion path executes — is what this file owns, so resolve.go stays
//	about the resolution entry point and its routability gate.
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

// resolveCombo resolves the combo's leading routable model and carries the
// combo's shape so the caller can execute the strategy it declares.
//
// The first entry that still resolves wins, and only a combo with no routable
// entry at all fails: a reference left behind by an edit must cost its own
// member, not the whole combo, and both strategies already skip members they
// cannot resolve. A fusion combo ignores the leading choice and fans out to
// every reference instead, which is why the strategy travels with the
// resolution rather than being re-read by the caller.
func (r *Resolver) resolveCombo(ctx context.Context, combo domain.Combo) (Resolution, error) {
	return r.resolveComboWithin(ctx, combo, 0)
}

// resolveComboWithin is resolveCombo with the depth a chain has already
// walked, so a nested member can re-enter resolution without losing the guard.
func (r *Resolver) resolveComboWithin(ctx context.Context, combo domain.Combo, depth int) (Resolution, error) {
	if combo.ModelCount() == 0 {
		return Resolution{}, dataPlaneError(CodeModelNotFound, "combo "+combo.Name()+" has no models")
	}
	if depth >= comboDepthLimit {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"combo "+combo.Name()+" is part of a chain longer than the resolver follows")
	}
	var lastErr error
	for _, ref := range combo.Refs() {
		resolved, err := r.resolveMember(ctx, ref, depth)
		if err != nil {
			lastErr = err
			continue
		}
		// The answered identity is the combo the client addressed, never an
		// inner one: the response identity, the rotation key, and the usage row
		// are all keyed by that name, and attributing the request to the inner
		// combo would spend the wrong rotation and record the wrong row.
		resolved.Combo = combo
		return resolved, nil
	}
	return Resolution{}, lastErr
}

// resolveMember resolves one combo entry. One dereference level only
// (SPEC-API-001 §7.7): a reference may be a provider/model reference, an
// alias, or a combo name, and a member that names a combo resolves to that
// combo's own leading member. The depth the chain has already walked travels
// with the call so a stored cycle terminates at comboDepthLimit instead of
// recursing without end (draft 024 §3.1).
func (r *Resolver) resolveMember(ctx context.Context, ref string, depth int) (Resolution, error) {
	// A combo is addressed by a bare name, so a reference carrying "/" cannot
	// be one; that also keeps a provider whose id collides with a combo name
	// from being shadowed, the same rule the entry point applies.
	if !hasSlash(ref) {
		combo, found, err := r.lookup.Combo(ctx, ref)
		if err != nil {
			return Resolution{}, err
		}
		if found {
			return r.resolveComboWithin(ctx, combo, depth+1)
		}
	}
	resolved, err := r.resolveReference(ctx, ref)
	if err == nil {
		return resolved, nil
	}
	target, ok, aliasErr := r.lookup.Alias(ctx, ref)
	if aliasErr != nil {
		return Resolution{}, aliasErr
	}
	if !ok {
		return Resolution{}, err
	}
	return r.resolveAliased(ctx, target, depth)
}

// resolveAliased follows an alias target at the member level, carrying the
// depth so a target naming a nested combo keeps the cycle guard. It mirrors the
// entry point's one-level rule: the target resolves, and it does not resolve as
// an alias again.
func (r *Resolver) resolveAliased(ctx context.Context, target string, depth int) (Resolution, error) {
	if !hasSlash(target) {
		combo, found, err := r.lookup.Combo(ctx, target)
		if err != nil {
			return Resolution{}, err
		}
		if found {
			return r.resolveComboWithin(ctx, combo, depth+1)
		}
	}
	return r.Resolve(ctx, target)
}

// resolveReference handles the provider/model form, where the first segment may
// be a provider id or any of its aliases.
func (r *Resolver) resolveReference(ctx context.Context, model string) (Resolution, error) {
	slash := indexOf(model, '/')
	if slash <= 0 || slash == len(model)-1 {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"model "+model+" is not a known model, alias, or combo")
	}
	return r.ResolveParts(ctx, model[:slash], model[slash+1:])
}

// hasSlash reports whether a model string carries the separator that makes it
// a provider/model reference. A bare name cannot be one, which is the rule that
// keeps a provider whose id collides with a combo name from being shadowed.
func hasSlash(value string) bool { return indexOf(value, '/') >= 0 }

// indexOf reports the position of sep, or -1. It keeps the resolver free of
// strings.Index arithmetic at the call sites, where an off-by-one would rename a
// model.
func indexOf(value string, sep byte) int {
	for i := 0; i < len(value); i++ {
		if value[i] == sep {
			return i
		}
	}
	return -1
}
