// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_dataplane.go
// @for       Registration of the §7.15 data-plane routes, split from the
//
//	management table so each file stays inside the AGENTS.md §1.1 budget.
//
// @uses      net/http.
// @reason    These routes are the one surface deliberately not session-gated, and
//
//	the reason is a credential fact rather than a style choice: a CLI
//	tool cannot hold a dashboard cookie. Keeping them in one function is
//	what makes that difference auditable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-09-19
package router

// registerDataPlaneRoutes registers the OpenAI, Anthropic and Responses wires,
// the models list, the token estimate, and embeddings. A CLI tool presents
// `Authorization: Bearer <gateway key>`, which the handler enforces when
// settings.security.require_api_key is true; a dashboard session cookie is not a
// credential a CLI tool can hold, so gating these routes on one would 401 every
// client request. A handler that was not built is not registered, so a deployment
// without one answers 404 rather than panicking on a nil handler.
func registerDataPlaneRoutes(mux *routeRecorder, deps Deps) {
	if deps.Chat != nil {
		mux.HandleFunc("POST "+APIVersion+"/chat/completions", deps.Chat.Completions)
		mux.HandleFunc("POST "+APIVersion+"/messages", deps.Chat.Messages)
		mux.HandleFunc("POST "+APIVersion+"/responses", deps.Chat.Responses)
		mux.HandleFunc("GET "+APIVersion+"/models", deps.Chat.Models)
	}
	if deps.Embeddings != nil {
		mux.HandleFunc("POST "+APIVersion+"/embeddings", deps.Embeddings.Embed)
	}
	// The estimate route needs the §4 key rule but not the engine.
	if deps.TokenCount != nil {
		mux.HandleFunc("POST "+APIVersion+"/messages/count_tokens", deps.TokenCount.Count)
	}
	// The decision route serves models declaring `kind: systemone`, whose payload
	// is the provider's own vocabulary rather than a chat body.
	if deps.SystemOne != nil {
		mux.HandleFunc("POST "+APIVersion+"/systemone", deps.SystemOne.Decide)
	}
}
