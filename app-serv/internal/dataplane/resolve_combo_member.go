// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_combo_member.go
// @for       The member walk: one combo entry, its alias hop, and the
//
//	provider/model reference form.
//
// @uses      internal/domain, context.
// @reason    The depth and expansion guards live with the combo walk; the member
//
//	walk that consumes them is a separate concern — a member may be a
//	reference, an alias, or a nested combo — so it sits here, apart from
//	the budget the caller owns. Separated at the AGENTS.md §1.1 line
//	limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
)

// resolveMember resolves one combo entry. One dereference level only
// (SPEC-API-001 §7.7): a reference may be a provider/model reference, an
// alias, or a combo name, and a member that names a combo resolves to that
// combo's own leading member. The depth the chain has already walked travels
// with the call so a stored cycle terminates at comboDepthLimit instead of
// recursing without end (draft 024 §3.1).
func (r *Resolver) resolveMember(ctx context.Context, ref string, depth, aliasHops int, state *resolveState) (Resolution, error) {
	// A combo is addressed by a bare name, so a reference carrying "/" cannot
	// be one; that also keeps a provider whose id collides with a combo name
	// from being shadowed, the same rule the entry point applies.
	if !hasSlash(ref) {
		combo, found, err := r.lookup.Combo(ctx, ref)
		if err != nil {
			return Resolution{}, err
		}
		if found {
			return r.resolveComboWithin(ctx, combo, depth+1, aliasHops, state)
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
	return r.resolveAliased(ctx, target, depth, aliasHops, state)
}

// resolveAliased follows an alias target at the member level, carrying the
// depth so a target naming a nested combo keeps the cycle guard. It mirrors the
// entry point's one-level rule: the target resolves, and it does not resolve as
// an alias again.
func (r *Resolver) resolveAliased(ctx context.Context, target string, depth, aliasHops int, state *resolveState) (Resolution, error) {
	if !hasSlash(target) {
		combo, found, err := r.lookup.Combo(ctx, target)
		if err != nil {
			return Resolution{}, err
		}
		if found {
			return r.resolveComboWithin(ctx, combo, depth+1, aliasHops, state)
		}
	}
	return r.resolveWithin(ctx, target, state, aliasHops+1)
}

// resolveReference handles the provider/model form, where the first segment may
// be a provider id or any of its aliases. It answers for the chat plane.
func (r *Resolver) resolveReference(ctx context.Context, model string) (Resolution, error) {
	return r.resolveReferenceKind(ctx, model, KindChat)
}

// resolveReferenceKind is the same split with the plane carried through, so a
// direct provider/model reference is filtered the way an alias chain is.
func (r *Resolver) resolveReferenceKind(ctx context.Context, model, kind string) (Resolution, error) {
	slash := indexOf(model, '/')
	if slash <= 0 || slash == len(model)-1 {
		return Resolution{}, dataPlaneError(CodeModelNotFound,
			"model "+model+" is not a known model, alias, or combo")
	}
	return r.ResolvePartsForKind(ctx, model[:slash], model[slash+1:], kind)
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
