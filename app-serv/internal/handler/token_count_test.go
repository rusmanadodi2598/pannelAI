// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/token_count_test.go
// @for       HTTP tests for POST /api/v1/messages/count_tokens.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy-path, a validation-failure, and an
//
//	auth-failure test per route. The estimate is pinned through the wire
//	as well, because the route's answer is a number a client trusts to
//	decide whether a request fits, and the key may arrive in either of
//	the two headers §4 accepts.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// tokenCountHandler wires the route over the §4 stub, so no engine is built to
// prove the key rule.
func tokenCountHandler(token string) *TokenCountHandler {
	return NewTokenCountHandler(service.NewTokenCountService(), stubAuthenticator{token: token})
}

// TestTokenCountHandler_Estimate drives the route over bodies whose text is
// known, so the answer is pinned end to end: decode, estimate, encode.
func TestTokenCountHandler_Estimate(t *testing.T) {
	handler := tokenCountHandler("")
	cases := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "a bare string body",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`,
			want: 2,
		},
		{
			name: "an exact token boundary",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"abcd"}]}`,
			want: 1,
		},
		{
			name: "no text",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"aGk="}}]}]}`,
			want: 0,
		},
		{
			name: "the members the route does not model are tolerated",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"betas":["prompt-caching"]}`,
			want: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodPost, "/api/v1/messages/count_tokens", tc.body, handler.Count)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
			}
			if got := decodeBody(t, rr)["input_tokens"]; got != tc.want {
				t.Fatalf("input_tokens = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTokenCountHandler_Validation pins the refusal path: a body that fails the
// Anthropic contract is a 400 in the §4 envelope, and the estimate is never
// computed for it.
func TestTokenCountHandler_Validation(t *testing.T) {
	handler := tokenCountHandler("")
	cases := []struct {
		name string
		body string
	}{
		{"a missing model", `{"messages":[{"role":"user","content":"hello"}]}`},
		{"a role outside the Anthropic pair", `{"model":"claude-sonnet-4","messages":[{"role":"system","content":"hello"}]}`},
		{"malformed JSON", `{"model":`},
		{"an empty body", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, http.MethodPost, "/api/v1/messages/count_tokens", tc.body, handler.Count)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if got := decodeBody(t, rr)["error"].(map[string]any)["code"]; got != "VALIDATION_ERROR" {
				t.Fatalf("code = %v, want VALIDATION_ERROR", got)
			}
		})
	}
}

// TestTokenCountHandler_Auth pins both credential headers the §4 rule accepts,
// and the refusal when neither carries the key.
func TestTokenCountHandler_Auth(t *testing.T) {
	handler := tokenCountHandler("good-key")
	body := `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`

	cases := []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{"the Authorization header", "Authorization", "Bearer good-key", http.StatusOK},
		{"the X-Api-Key header", "X-Api-Key", "good-key", http.StatusOK},
		{"no credential", "", "", http.StatusUnauthorized},
		{"a wrong credential", "Authorization", "Bearer nope", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/messages/count_tokens", strings.NewReader(body))
			if tc.header != "" {
				request.Header.Set(tc.header, tc.value)
			}
			rr := httptest.NewRecorder()
			handler.Count(rr, request)
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.want, rr.Body.String())
			}
			if tc.want == http.StatusUnauthorized {
				if got := decodeBody(t, rr)["error"].(map[string]any)["code"]; got != "UNAUTHORIZED" {
					t.Fatalf("code = %v, want UNAUTHORIZED", got)
				}
			}
		})
	}
}
