// Package router maps HTTP routes to handlers.
//
// @file      internal/router/route_recorder.go
// @for       Pattern recording for the mux, feeding the §7.17 contract test.
// @uses      net/http.
// @reason    The served openapi.json must name every registered route, so the
//
//	mux's table and the contract come from one list. Recording at
//	registration keeps that true by construction; splitting the
//	recorder out of router.go keeps the registration table inside its
//	AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-20
package router

import "net/http"

// Routes returns every registered pattern in registration order, as
// "METHOD path". The §7.17 contract test walks it against the served
// openapi.json, which is what makes a route missing from the document a build
// failure rather than a doc lag.
func (m *Mux) Routes() []string { return m.routes }

// routeRecorder records each pattern as it registers, so the mux's table and
// the served contract come from one list rather than two.
type routeRecorder struct {
	*http.ServeMux
	patterns []string
}

func (r *routeRecorder) Handle(pattern string, handler http.Handler) {
	r.patterns = append(r.patterns, pattern)
	r.ServeMux.Handle(pattern, handler)
}

func (r *routeRecorder) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.Handle(pattern, handler)
}
