// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/resolve_disabled.go
// @for       The operator disabled set applied to the routing path.
// @uses      context, internal/domain (through the ModelLookup the resolver holds).
// @reason    SPEC-API-001 §7.6 makes a disabled pair hidden from routing, and a rule the listing applies while the router ignores it is a rule an operator cannot rely on.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package dataplane

import "context"

// refuseIfDisabled refuses a model the operator hid, naming it the way the catalog
// does so a hidden model reads as an absent one rather than as a policy the client
// should know about. The listing filters the whole disabled set in one read; routing
// asks about the single pair it is about to use, because it already knows the
// provider. A read that fails is treated as not disabled: the disabled set is an
// operator preference, so a catalog blip must not lock every provider out, and the
// cost of failing open is one served call the operator meant to hide, corrected the
// moment the read works again.
func (r *Resolver) refuseIfDisabled(ctx context.Context, providerID, modelID string) error {
	disabled, err := r.lookup.Disabled(ctx, providerID, modelID)
	if err != nil {
		//nolint:nilerr // reason: the fail-open documented above; a catalog read that fails must not lock every provider out, and the caller has no second answer to give.
		return nil
	}
	if !disabled {
		return nil
	}
	return dataPlaneError(CodeModelNotFound, "model "+providerID+"/"+modelID+" is not available")
}
