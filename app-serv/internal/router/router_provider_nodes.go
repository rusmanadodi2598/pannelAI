// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_provider_nodes.go
// @for       The §7.4 custom provider node and credential-check routes.
// @uses      internal/handler, net/http.
// @reason    §7.4 carries seven routes once the stateless credential checks land
//
//	(draft 017 §4.6), which pushed router.go past the AGENTS.md §1.1 line
//	budget. They are split out rather than the file trimmed, because the
//	node routes are one cohesive group: everything here answers a question
//	about a provider the operator defined themselves.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-23
package router

import "net/http"

// registerProviderNodeRoutes registers the §7.4 node routes and the two
// stateless credential checks, all session-gated like the rest of management.
//
// The two validate routes are deliberately stateless: their bodies carry the
// destination and the credential, so an operator can prove a key before the node
// or endpoint exists (draft 017 §4.6).
func registerProviderNodeRoutes(mux *routeRecorder, deps Deps, gateway func(http.Handler) http.Handler) {
	mux.Handle("GET "+APIVersion+"/provider-nodes", gateway(http.HandlerFunc(deps.ProviderNode.List)))
	mux.Handle("POST "+APIVersion+"/provider-nodes", gateway(http.HandlerFunc(deps.ProviderNode.Create)))
	mux.Handle("GET "+APIVersion+"/provider-nodes/{id}", gateway(http.HandlerFunc(deps.ProviderNode.Get)))
	mux.Handle("PATCH "+APIVersion+"/provider-nodes/{id}", gateway(http.HandlerFunc(deps.ProviderNode.Update)))
	mux.Handle("DELETE "+APIVersion+"/provider-nodes/{id}", gateway(http.HandlerFunc(deps.ProviderNode.Delete)))
	mux.Handle("POST "+APIVersion+"/provider-nodes/{id}/test", gateway(http.HandlerFunc(deps.ProviderNode.Test)))
	// §7.4 credential checks. Both are stateless: the body carries the
	// destination and the credential, and no row is written (draft 017 §4.6).
	// They sit beside the node routes because that is where an operator asks
	// "does this credential work" — before the node or the endpoint exists.
	mux.Handle("POST "+APIVersion+"/provider-nodes/validate", gateway(http.HandlerFunc(deps.ProviderValidate.Node)))
	mux.Handle("POST "+APIVersion+"/providers/validate", gateway(http.HandlerFunc(deps.ProviderValidate.Provider)))
}
