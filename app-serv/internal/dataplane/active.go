// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/active.go
// @for       The in-flight marker seam the relay leg opens before its outbound
//
//	call and closes after it.
//
// @uses      context.
// @reason    SPEC-UI-001 §6.5 makes the drawing's active node mean "this
//
//	provider is being called right now", and the relay leg is the only
//	place in the data plane that knows both the provider and the
//	interval. The seam is declared here rather than reached for directly
//	so the engine keeps no Redis dependency (AGENTS.md §1.5): the
//	composition root adapts the store to it.
//
//	Begin returns the release rather than taking a matching End, because
//	the two halves cannot be separated that way: a caller that has the
//	release has the obligation, and the leg defers it immediately.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package dataplane

import "context"

// ActiveRequests records one provider as being called for as long as the call
// runs. A nil seam records nothing, which is the documented behaviour for a
// deployment that wired no broker.
type ActiveRequests interface {
	// Begin marks providerID as in flight and returns the function that
	// releases it. The returned function is safe to call more than once.
	Begin(ctx context.Context, providerID, endpointID, model string) func()
}

// markActive opens one marker and returns its release.
//
// It is the nil-safe form of the seam, because a typed nil inside an interface
// still panics when its method is called: the engine holds the seam as an
// interface, so a deployment that wired no tracker would reach a nil receiver
// rather than the no-op the seam promises. Returning a release here keeps that
// rule in one place instead of at every call site.
func (e *Engine) markActive(ctx context.Context, providerID, endpointID, model string) func() {
	if e == nil || e.active == nil {
		return func() {}
	}
	return e.active.Begin(ctx, providerID, endpointID, model)
}
