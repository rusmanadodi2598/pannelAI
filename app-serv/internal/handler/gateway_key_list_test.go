// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key_list_test.go
// @for       HTTP tests for GET /api/v1/gateway-keys pagination.
// @uses      internal/handler, internal/schema, internal/service, internal/domain,
//            internal/repository, net/http/httptest.
// @reason    SPEC-API-001 §4 fixes the meta block (page, per_page, total) and the 100 cap; these cases cover defaults, mid-page, past-the-end, and clamped values so the bound is enforced rather than assumed.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-16

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestGatewayKey_List_Pagination verifies the meta block and that slicing
// follows the requested page. per_page above the cap is a refusal rather than
// a clamp (draft 010 F6, owner decision D3), and that refusal is pinned in
// TestGatewayKey_List_InvalidPage beside the other out-of-range values.
func TestGatewayKey_List_Pagination(t *testing.T) {
	h, repo := newTestHandler(t)
	for i := 0; i < 3; i++ {
		if err := repo.Create(context.Background(), domain.NewGatewayKey(
			"key-"+string(rune('a'+i)), "sk-secret-"+string(rune('a'+i)), "sk-…wxyz", time.Now())); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	cases := []struct {
		name        string
		query       string
		wantLen     int
		wantPerPage int
	}{
		{"defaults", "", 3, 25},
		{"limited page", "?per_page=2", 2, 2},
		{"second page", "?per_page=2&page=2", 1, 2},
		{"page past the end", "?per_page=2&page=9", 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/gateway-keys"+tc.query, nil)
			rr := httptest.NewRecorder()
			h.List(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
			}
			var out struct {
				Data []map[string]any `json:"data"`
				Meta struct {
					Page    int   `json:"page"`
					PerPage int   `json:"per_page"`
					Total   int64 `json:"total"`
				} `json:"meta"`
			}
			if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(out.Data) != tc.wantLen {
				t.Fatalf("len(data) = %d, want %d", len(out.Data), tc.wantLen)
			}
			if out.Meta.PerPage != tc.wantPerPage {
				t.Fatalf("per_page = %d, want %d", out.Meta.PerPage, tc.wantPerPage)
			}
			if out.Meta.Total != 3 {
				t.Fatalf("total = %d, want 3", out.Meta.Total)
			}
			for _, row := range out.Data {
				if _, leaked := row["plaintext_key"]; leaked {
					t.Fatal("list must never return plaintext keys")
				}
			}
		})
	}
}

// TestGatewayKey_List_InvalidPage maps a malformed page to VALIDATION_ERROR.
func TestGatewayKey_List_InvalidPage(t *testing.T) {
	h, _ := newTestHandler(t)

	cases := []struct {
		name  string
		query string
	}{
		{"zero page", "?page=0"},
		{"negative page", "?page=-3"},
		{"non-numeric page", "?page=abc"},
		{"zero per_page", "?per_page=0"},
		{"non-numeric per_page", "?per_page=x"},
		{"per_page one over the maximum", "?per_page=101"},
		{"per_page far over the maximum", "?per_page=99999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/gateway-keys"+tc.query, nil)
			rr := httptest.NewRecorder()
			h.List(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			body := decodeBody(t, rr)
			errObj, _ := body["error"].(map[string]any)
			if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
				t.Fatalf("code = %q, want VALIDATION_ERROR", code)
			}
		})
	}
}

// TestGatewayKey_Revoke_IsTerminal covers DELETE then a second revoke.
