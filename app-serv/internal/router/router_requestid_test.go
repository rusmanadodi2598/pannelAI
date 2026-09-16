// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_requestid_test.go
// @for       The §4 requirement that every request carries a request id.
// @uses      internal/router, internal/schema, net/http, net/http/httptest,
//
//	strings, encoding/json, testing.
//
// @reason    SPEC-API-001 §8 requires an INTERNAL_ERROR to be logged with a request id, so the id must exist and must survive when a caller supplies one.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRoutes_RequestIDIsEchoed verifies the §4 requirement that every request
// carries a request id, keeping a caller-supplied one when present.
func TestRoutes_RequestIDIsEchoed(t *testing.T) {
	mux := newTestRouter(t)

	t.Run("generated when absent", func(t *testing.T) {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/version", nil))
		if got := rr.Header().Get(RequestIDHeader); got == "" {
			t.Fatal("response must carry a generated request id")
		}
	})

	t.Run("caller-supplied id is preserved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
		req.Header.Set(RequestIDHeader, "caller-trace-123")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if got := rr.Header().Get(RequestIDHeader); got != "caller-trace-123" {
			t.Fatalf("request id = %q, want the caller's value", got)
		}
	})
}
