// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/chat_auth_precedence_test.go
// @for       The §7.15 pipeline order: authentication is settled before the
// request body is read or validated.
// @uses      net/http, net/http/httptest, strings, testing.
// @reason    F2 of docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md found the
// handler decoding before authenticating, so a malformed body answered
// VALIDATION_ERROR to a caller who had presented no credential. SPEC-API-001
// §7.15 fixes auth first, and OWASP A07 requires the credential gate to run
// before any protected operation, including a schema read that would otherwise
// let an anonymous caller probe the contract.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestChatCompletionsHTTP_AuthPrecedence pins the order as a property of the
// response: the status is decided by the credential, and the body only decides
// the answer once the credential is accepted.
func TestChatCompletionsHTTP_AuthPrecedence(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	cases := []struct {
		name       string
		key        string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "no key with a malformed body", body: "not-json", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "no key with a valid body", body: chatBody("test/model"), wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "no key with an empty body", body: "", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "no key with an oversized body", body: strings.Repeat("a", 9<<20), wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "a wrong key with a malformed body", key: "sk-wrong", body: "not-json", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "a wrong key with a valid body", key: "sk-wrong", body: chatBody("test/model"), wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
		{name: "a valid key with a malformed body", key: fixture.key, body: "not-json", wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR"},
		{name: "a valid key with an empty body", key: fixture.key, body: "", wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR"},
		{name: "a valid key with an oversized body", key: fixture.key, body: strings.Repeat("a", 9<<20), wantStatus: http.StatusBadRequest, wantCode: "VALIDATION_ERROR"},
		{name: "a valid key with a valid body", key: fixture.key, body: chatBody("test/model"), wantStatus: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(tc.body))
			if tc.key != "" {
				request.Header.Set("Authorization", "Bearer "+tc.key)
			}
			recorder := httptest.NewRecorder()
			handler.Completions(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("body = %s, want code %s", recorder.Body.String(), tc.wantCode)
			}
		})
	}
}

// TestChatCompletionsHTTP_RejectedCallerIsNotEchoed pins the second half of the
// same rule: a refusal must not quote the body back, because the body of a
// rejected caller is exactly what an unauthenticated prober would use to learn
// the contract one field at a time.
func TestChatCompletionsHTTP_RejectedCallerIsNotEchoed(t *testing.T) {
	fixture := newChatHTTPFixture(t)
	handler := NewChatHandler(fixture.chat)
	const probe = "probe-marker-value"
	body := `{"model":"` + probe + `","messages":[{"role":"banana","content":"` + probe + `"}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.Completions(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), probe) {
		t.Fatalf("the refusal echoed the caller's body: %s", recorder.Body.String())
	}
}
