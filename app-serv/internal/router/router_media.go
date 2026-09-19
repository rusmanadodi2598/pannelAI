// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_media.go
// @for       Route registration for the §7.10 media surface.
// @uses      internal/handler, net/http.
// @reason    §7.10 is the one section with both planes: the provider
//
//	configuration is management (session-gated) and the media calls
//	themselves are data plane (gateway key). Keeping them in one
//	registration keeps the section auditable in one place without
//	pushing router.go toward the §1.1 line limit.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import "net/http"

// registerMediaRoutes wires the media provider routes. The three
// /media-providers routes are management routes and share the session guard the
// other management routes use; the media calls themselves are data plane and
// deliberately NOT session-gated, for the same reason the chat wires are not: a
// CLI tool presents `Authorization: Bearer <gateway key>`, and a dashboard
// session cookie is not a credential a CLI tool can hold.
func registerMediaRoutes(mux *routeRecorder, deps Deps, gateway func(http.Handler) http.Handler) {
	mux.Handle("GET "+APIVersion+"/media-providers", gateway(http.HandlerFunc(deps.MediaProvider.List)))
	mux.Handle("GET "+APIVersion+"/media-providers/{provider_id}", gateway(http.HandlerFunc(deps.MediaProvider.Get)))
	mux.Handle("PATCH "+APIVersion+"/media-providers/{provider_id}", gateway(http.HandlerFunc(deps.MediaProvider.Patch)))

	if deps.Media == nil {
		return
	}
	mux.HandleFunc("POST "+APIVersion+"/audio/speech", deps.Media.Speech)
	mux.HandleFunc("POST "+APIVersion+"/audio/transcriptions", deps.Media.Transcribe)
	mux.HandleFunc("GET "+APIVersion+"/audio/voices", deps.Media.Voices)
	mux.HandleFunc("POST "+APIVersion+"/images/generations", deps.Media.Images)
	mux.HandleFunc("POST "+APIVersion+"/videos/generations", deps.Media.Videos)
	mux.HandleFunc("POST "+APIVersion+"/search", deps.Media.Search)
}
