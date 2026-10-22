// Package router maps HTTP routes to handlers.
//
// @file      internal/router/security_headers_test.go
// @for       The response headers every route carries, including the one-time key response.
// @uses      net/http, net/http/httptest, testing.
// @reason    A management response can carry a plaintext key or a proxy URL, so no-store and nosniff must be applied by the chain rather than remembered per handler.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-22
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChainAppliesSecurityHeaders(t *testing.T) {
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}
