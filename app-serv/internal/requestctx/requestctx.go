// Package requestctx carries the per-request identity every layer may log against.
//
// @file      internal/requestctx/requestctx.go
// @for       Storing and reading the request id across layers.
// @uses      context
// @reason    The router sets the id and the handlers are the ones that log failures worth
//
//	tracing, but a handler reaching into the router for the accessor would import upward
//	against AGENTS.md §1.5. The key therefore lives below both of them, in the one place
//	that owns no layer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-04
package requestctx

import "context"

type requestIDKey struct{}

// With returns a context carrying the request id the middleware generated.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// From answers the request id on this context, or "" when no middleware ran.
// An empty id is a valid answer, not an error: the recovery path and tests build
// bare contexts, and a log line without an id is worse than none at all.
func From(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
