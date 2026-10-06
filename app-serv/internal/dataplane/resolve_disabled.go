// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_disabled.go
// @for       The operator disabled set applied to the routing path.
// @uses      context, internal/domain and internal/registry (through the ModelLookup the resolver holds).
// @reason    SPEC-API-001 §7.6 makes a disabled pair hidden from routing, and a rule the listing applies while the router ignores it is a rule an operator cannot rely on.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// refuseIfDisabled refuses a model the operator hid, naming it the way the catalog
// does so a hidden model reads as an absent one rather than as a policy the client
// should know about. The question is the listing's own, over every spelling the
// entry answers to: a disable written as an alias or a node prefix already hid the
// model from /models, and matching only the canonical id would keep serving and
// billing it. A read that fails is treated as not disabled: the disabled set is an
// operator preference, so a catalog blip must not lock every provider out, and the
// cost of failing open is one served call the operator meant to hide, corrected the
// moment the read works again.
func (r *Resolver) refuseIfDisabled(ctx context.Context, entry registry.Provider, modelID string) error {
	refs, err := r.lookup.DisabledPairs(ctx)
	if err != nil {
		//nolint:nilerr // reason: the fail-open documented above; a catalog read that fails must not lock every provider out, and the caller has no second answer to give.
		return nil
	}
	if !r.isDisabled(disabledIndex(refs), entry, modelID) {
		return nil
	}
	return dataPlaneError(CodeModelNotFound, "model "+entry.ID+"/"+modelID+" is not available")
}
