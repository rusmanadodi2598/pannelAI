// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_lifecycle_test.go
// @for       The full create, read, revoke HTTP lifecycle through the mux.
// @uses      internal/router, internal/schema, net/http, net/http/httptest,
//
//	strings, encoding/json, testing.
//
// @reason    The individual endpoints can each pass while the lifecycle between them is broken; this walks create, get, and delete as one flow.
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRoutes_CreateThenGetThroughMux drives create, read, and revoke as one flow
// through the real mux: create returns the plaintext once, GET returns only the
// hint, and DELETE revokes. Each endpoint can pass alone while the lifecycle
// between them is broken, which is what this covers.
func TestRoutes_CreateThenGetThroughMux(t *testing.T) {
	mux := newTestRouter(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(`{"name":"ci"}`))
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201 (body: %s)", createRR.Code, createRR.Body.String())
	}

	var created map[string]any
	if err := json.NewDecoder(createRR.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	id, _ := created["id"].(string)
	plaintext, _ := created["plaintext_key"].(string)
	if id == "" || plaintext == "" {
		t.Fatalf("create must return id and plaintext: %v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/gateway-keys/"+id, nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("get = %d, want 200 (body: %s)", getRR.Code, getRR.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(getRR.Body).Decode(&got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if _, leaked := got["plaintext_key"]; leaked {
		t.Fatal("GET must not return the plaintext key")
	}
	if got["key_hint"] == nil {
		t.Fatal("GET must return the key hint")
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/gateway-keys/"+id, nil)
	delRR := httptest.NewRecorder()
	mux.ServeHTTP(delRR, delReq)
	if delRR.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204 (body: %s)", delRR.Code, delRR.Body.String())
	}
}
