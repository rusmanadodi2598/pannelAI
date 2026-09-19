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
	if combo.ModelCount() == 0 {
		return Resolution{}, dataPlaneError(CodeModelNotFound, "combo "+combo.Name()+" has no models")
	}
	var lastErr error
	for _, ref := range combo.Refs() {
		resolved, err := r.resolveMember(ctx, ref)
		if err != nil {
			lastErr = err
			continue
		}
		resolved.Combo = combo
		return resolved, nil
	}
	return Resolution{}, lastErr
}

// resolveMember resolves one combo entry. One dereference level only
// (SPEC-API-001 §7.7): a reference may be a provider/model or an alias, and a
// nested combo is refused rather than expanded, because a cycle between two
// combos would otherwise recurse.
func (r *Resolver) resolveMember(ctx context.Context, ref string) (Resolution, error) {
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
