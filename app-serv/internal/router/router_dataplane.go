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
// @stability experimental
// @since     2026-09-19
package router

// registerDataPlaneRoutes registers §7.15: the OpenAI, Anthropic, and Responses
// wires, the models list, the token estimate, and embeddings.
//
// A CLI tool presents `Authorization: Bearer <gateway key>` and the handler
// enforces it when settings.security.require_api_key is true (§4); the
// dashboard session cookie is not a credential a CLI tool can hold, so wrapping
// these in the session guard would make every client request a 401.
//
// A handler that was not built is simply not registered, so a deployment
// without one answers 404 there rather than panicking on a nil handler.
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
}
