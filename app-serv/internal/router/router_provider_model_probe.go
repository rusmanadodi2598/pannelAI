// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_provider_model_probe.go
// @for       The §7.4 model test routes (draft 017 §4.10, F10).
// @uses      internal/handler, net/http.
// @reason    These two routes are registered apart because §7.4's provider block in router.go is at the AGENTS.md §1.1 budget, and they are a group of their own by nature: both ask the data plane a question about one model, which is the question the connectivity tests never answered.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-27
package router

import "net/http"

// registerProviderModelTestRoutes registers the two model test routes, both
// session-gated like the rest of management.
//
// The single-model route is addressed under `/models/test` rather than
// `/models/{model_id}/test` because a model id is not a stored resource: it is
// the string the operator would put in a client, and it may contain slashes and
// dots that a path segment has to carry escaped.
func registerProviderModelTestRoutes(mux *routeRecorder, deps Deps, gateway func(http.Handler) http.Handler) {
	mux.Handle("POST "+APIVersion+"/providers/{provider_id}/models/test",
		gateway(http.HandlerFunc(deps.ModelProbe.Model)))
	mux.Handle("POST "+APIVersion+"/providers/{provider_id}/test-models",
		gateway(http.HandlerFunc(deps.ModelProbe.Models)))
}
