// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key_read_test.go
// @for       HTTP tests for GET /api/v1/gateway-keys/{id}.
// @uses      internal/handler, internal/schema, internal/service, internal/domain,
//            internal/repository, net/http/httptest.
// @reason    AGENTS.md §2.1 requires a happy path and an auth/error path per route; SPEC-API-001 §4 forbids returning the secret after creation, so the read path must be proven hint-only.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-16

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGatewayKey_Get_AfterCreate shows only the hint, never the plaintext.
func TestGatewayKey_Get_AfterCreate(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(`{"name":"ci-runner"}`))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	created := decodeBody(t, rr)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create must return an id")
	}

	getReq := setPathID(httptest.NewRequest(http.MethodGet, "/api/v1/gateway-keys/"+id, nil), id)
	getRR := httptest.NewRecorder()
	h.Get(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getRR.Code)
	}
	got := decodeBody(t, getRR)
	if _, leaked := got["plaintext_key"]; leaked {
		t.Fatal("GET must never return the plaintext key")
	}
	if got["id"] != id {
		t.Fatalf("id = %v, want %q", got["id"], id)
	}
}

// TestGatewayKey_Get_NotFound maps an unknown id to 404 NOT_FOUND.
func TestGatewayKey_Get_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	req := setPathID(httptest.NewRequest(http.MethodGet, "/api/v1/gateway-keys/gky_missing", nil), "gky_missing")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if code, _ := errObj["code"].(string); code != "NOT_FOUND" {
		t.Fatalf("code = %q, want NOT_FOUND", code)
	}
}

// TestGatewayKey_List_Pagination verifies the meta block, the clamped
