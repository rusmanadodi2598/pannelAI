// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router.go
// @for       Route registration for the v1 management and data planes.
// @uses      internal/handler, internal/schema, net/http, encoding/json.
// @reason    SPEC-API-001 §4 pins the /api/v1 prefix for every route and §9
//
//	fixes the layer flow ending at the router; registering each route
//	with its method makes the surface auditable at a glance and makes
//	a forgotten verb a startup-visible mistake, not a 405 at runtime.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"encoding/json"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// APIVersion is the single version prefix for all routes (SPEC-API-001 §11.1).
const APIVersion = "/api/v1"

// Mux is the HTTP serve mux for app-serv.
type Mux struct {
	http.Handler
}

// Deps holds the handlers the router wires up.
type Deps struct {
	System     *handler.SystemHandler
	GatewayKey *handler.GatewayKeyHandler
}

// New registers every route and returns the assembled mux. Patterns carry the
// method (Go 1.22 ServeMux syntax) so the mux rejects a wrong verb before any
// handler sees it.
func New(deps Deps) *Mux {
	mux := http.NewServeMux()

	// §7.1 System (public).
	mux.HandleFunc("GET "+APIVersion+"/health", deps.System.Health)
	mux.HandleFunc("GET "+APIVersion+"/version", deps.System.Version)

	// §7.3 Gateway keys.
	mux.HandleFunc("GET "+APIVersion+"/gateway-keys", deps.GatewayKey.List)
	mux.HandleFunc("POST "+APIVersion+"/gateway-keys", deps.GatewayKey.Create)
	mux.HandleFunc("GET "+APIVersion+"/gateway-keys/{id}", deps.GatewayKey.Get)
	mux.HandleFunc("PATCH "+APIVersion+"/gateway-keys/{id}", deps.GatewayKey.Update)
	mux.HandleFunc("DELETE "+APIVersion+"/gateway-keys/{id}", deps.GatewayKey.Delete)

	// Unknown paths and wrong verbs stay the mux's answer so it can distinguish
	// 404 from 405 (and send Allow on the latter); envelope() then restates
	// either in the §8 shape, so routing errors look like every other error.
	return &Mux{Handler: chain(mux)}
}

// envelope restates the mux's own plain-text 404/405 answers in the management
// error envelope (SPEC-API-001 §8). It intercepts only responses no handler
// produced: a handler always sets a JSON content type before writing, so
// anything already JSON passes through untouched and is never double-bodied.
func envelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if !rec.suppressed {
			return
		}
		switch rec.status {
		case http.StatusNotFound:
			writeEnvelope(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		case http.StatusMethodNotAllowed:
			writeEnvelope(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed for this route")
		}
	})
}

// writeEnvelope emits the §8 error shape for a response the mux produced and
// this middleware suppressed, so no other body exists for that request.
func writeEnvelope(w http.ResponseWriter, code int, errCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(schema.ErrorBody{Error: schema.ErrorDetail{Code: errCode, Message: msg}})
}

// statusRecorder intercepts only the mux's built-in 404/405 answers. Those are
// the responses that carry a text/plain content type; a handler response always
// sets application/json first, so it is forwarded verbatim.
type statusRecorder struct {
	http.ResponseWriter
	status     int
	suppressed bool
}

func (rec *statusRecorder) WriteHeader(code int) {
	if rec.suppressed {
		return
	}
	builtin := code == http.StatusNotFound || code == http.StatusMethodNotAllowed
	if builtin && rec.Header().Get("Content-Type") != "application/json" {
		// Withhold the mux's plain-text body so envelope() can replace it.
		rec.suppressed = true
		rec.status = code
		return
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.suppressed {
		return len(b), nil
	}
	return rec.ResponseWriter.Write(b)
}
