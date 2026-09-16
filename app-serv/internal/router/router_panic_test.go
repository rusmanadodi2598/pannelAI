// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_panic_test.go
// @for       Recovery behavior when a handler panics mid-request.
// @uses      internal/router, internal/schema, net/http, net/http/httptest,
//
//	encoding/json, testing.
//
// @reason    AGENTS.md §1.6 makes panic recovery at every boundary
//
//	non-negotiable: an unrecovered panic takes down the whole binary,
//	not just the request. That guarantee is the one a request cannot
//	verify by succeeding, so it is asserted here directly, along with
//	the §8 requirement that the failure is reported with a request id.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestRecoverer_ReturnsEnvelopeForPanic drives a panicking handler through the
// same middleware chain the router builds.
func TestRecoverer_ReturnsEnvelopeForPanic(t *testing.T) {
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	handler := chain(panicking)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/whatever", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	dec := json.NewDecoder(rr.Body)
	var envelope schema.ErrorBody
	if err := dec.Decode(&envelope); err != nil {
		t.Fatalf("body is not one JSON document: %v (body: %q)", err, rr.Body.String())
	}
	if envelope.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("code = %q, want INTERNAL_ERROR", envelope.Error.Code)
	}

	// A second document after the first would mean the recovered response was
	// appended to rather than written once.
	var extra map[string]any
	if err := dec.Decode(&extra); err == nil {
		t.Fatalf("body carried a second JSON document: %q", rr.Body.String())
	}

	// §8 requires an INTERNAL_ERROR be traceable, so the id must survive the
	// panic path; the middleware sets it before the handler runs.
	if got := rr.Header().Get(RequestIDHeader); got == "" {
		t.Fatal("a recovered 500 must still carry a request id")
	}
}

// TestRecoverer_ProcessSurvivesPanic proves the panic does not escape the
// chain: the call returns normally and a later request is still served.
func TestRecoverer_ProcessSurvivesPanic(t *testing.T) {
	calls := 0
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			panic("first request only")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/a", nil))
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first status = %d, want 500", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/v1/b", nil))
	if second.Code != http.StatusNoContent {
		t.Fatalf("second status = %d, want 204: the chain did not survive the panic", second.Code)
	}
}
