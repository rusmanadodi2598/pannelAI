// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key_rename_test.go
// @for       HTTP tests for a rename that collides with another key's name.
// @uses      internal/handler, internal/schema, net/http, net/http/httptest,
//
//	strings, encoding/json, testing.
//
// @reason    The repository contract rejects a duplicate name, and a rename is a
//
//	different statement from a create; SPEC-API-001 §7.3 exposes both.
//	The stub mirrors the UNIQUE index so this covers the mapping from
//	domain.ErrGatewayKeyExists to a 409 for the PATCH path.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     handler
// @stability experimental
// @since     2026-09-16
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestGatewayKey_Update_RenameCollision posts two keys, then renames the second
// onto the first one's name and expects 409 CONFLICT.
func TestGatewayKey_Update_RenameCollision(t *testing.T) {
	h, _ := newTestHandler(t)

	create := func(name string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys",
			strings.NewReader(`{"name":"`+name+`"}`))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create %q = %d, want 201 (body: %s)", name, rr.Code, rr.Body.String())
		}
		body := decodeBody(t, rr)
		id, _ := body["id"].(string)
		return id
	}

	create("owner")
	second := create("borrower")

	patch := httptest.NewRequest(http.MethodPatch, "/api/v1/gateway-keys/"+second,
		strings.NewReader(`{"name":"owner"}`))
	patch.SetPathValue("id", second)
	rr := httptest.NewRecorder()
	h.Update(rr, patch)

	if rr.Code != http.StatusConflict {
		t.Fatalf("rename collision = %d, want 409 (body: %s)", rr.Code, rr.Body.String())
	}
	var envelope schema.ErrorBody
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Error.Code != "CONFLICT" {
		t.Fatalf("code = %q, want CONFLICT", envelope.Error.Code)
	}
}
