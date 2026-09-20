// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key_create_test.go
// @for       HTTP tests for POST /api/v1/gateway-keys.
// @uses      internal/handler, internal/schema, internal/service, internal/domain,
//            internal/repository, net/http/httptest.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path for every route; SPEC-API-001 §7.3 requires the plaintext to be returned once and the hint thereafter, which is the contract a leak would break.
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

// TestGatewayKey_Create_HappyPath returns the plaintext exactly once and
// reports the masking hint, not the secret.
func TestGatewayKey_Create_HappyPath(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(`{"name":"ci-runner"}`))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	plaintext, _ := body["plaintext_key"].(string)
	if plaintext == "" {
		t.Fatal("create must return the plaintext key once")
	}
	if !strings.HasPrefix(plaintext, "sk-") {
		t.Fatalf("plaintext %q must carry the configured prefix", plaintext)
	}
	hint, _ := body["key_hint"].(string)
	if !strings.HasPrefix(hint, "sk-…") {
		t.Fatalf("key_hint %q must be masked (sk-…abcd)", hint)
	}
	// The hint may only expose the family prefix plus the trailing characters;
	// it must not contain any interior run of the secret body.
	tail := plaintext[len(plaintext)-4:]
	body_ := plaintext[len("sk-") : len(plaintext)-4]
	if len(body_) > 4 && strings.Contains(hint, body_[1:len(body_)-1]) {
		t.Fatalf("key_hint %q must not leak the secret body of %q", hint, plaintext)
	}
	if !strings.HasSuffix(hint, tail) {
		t.Fatalf("key_hint %q must end with the trailing characters %q", hint, tail)
	}
}

// TestGatewayKey_Create_Validation covers the schema validation failures.
func TestGatewayKey_Create_Validation(t *testing.T) {
	h, _ := newTestHandler(t)

	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{}`},
		{"empty name", `{"name":""}`},
		{"name too long", `{"name":"` + strings.Repeat("a", 121) + `"}`},
		{"malformed json", `{"name":}`},
		{"unknown field", `{"name":"ci","extra":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			h.Create(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			body := decodeBody(t, rr)
			errObj, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("response must carry the error envelope: %v", body)
			}
			if code, _ := errObj["code"].(string); code != "VALIDATION_ERROR" {
				t.Fatalf("code = %q, want VALIDATION_ERROR", code)
			}
			if msg, _ := errObj["message"].(string); msg == "" {
				t.Fatal("error message must not be empty")
			}
		})
	}
}

// TestGatewayKey_Create_DuplicateNameConflict pins the §6 uniqueness rule's
// journey: the first create succeeds, a second create with the same name is a
// 409 CONFLICT rather than a 500 or a validation error, and a create with a
// different name still succeeds afterward, so the refusal tracks the name and
// not the fact that a key exists.
func TestGatewayKey_Create_DuplicateNameConflict(t *testing.T) {
	h, _ := newTestHandler(t)

	create := func(name string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway-keys", strings.NewReader(`{"name":"`+name+`"}`))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		return rr
	}

	if first := create("ci"); first.Code != http.StatusCreated {
		t.Fatalf("first create = %d, want 201 (body: %s)", first.Code, first.Body.String())
	}
	duplicate := create("ci")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409 (body: %s)", duplicate.Code, duplicate.Body.String())
	}
	body := decodeBody(t, duplicate)
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response must carry the error envelope: %v", body)
	}
	if code, _ := errObj["code"].(string); code != "CONFLICT" {
		t.Fatalf("code = %q, want CONFLICT", code)
	}
	if again := create("cd"); again.Code != http.StatusCreated {
		t.Fatalf("create after the conflict = %d, want 201 (body: %s)", again.Code, again.Body.String())
	}
}
