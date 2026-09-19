// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_callback_guard_test.go
// @for       The callback's redirect-origin guard and its no-origin answer.
// @uses      net/http, net/http/httptest, testing.
// @reason    §7.4 sends a browser back to the panel after an authorization, and
//
//	that destination must never come from the request. The guard is
//	tested directly rather than through a flow so every refused value is
//	pinned, and the no-origin case is pinned separately because it is the
//	deployment with no configured base URL: it answers JSON instead of
//	guessing a host (OWASP A01).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOAuthHandlerRedirectOriginRefusesUnsafeValues pins the helper's contract
// directly: only an absolute http(s) origin survives, so no future caller can
// turn the callback into an open redirect.
func TestOAuthHandlerRedirectOriginRefusesUnsafeValues(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		staged  string
		want    string
	}{
		{name: "the configured base URL is authoritative",
			baseURL: "https://gateway.example.com", staged: "https://evil.example", want: "https://gateway.example.com"},
		{name: "a staged origin is used when nothing is configured",
			staged: "https://panel.example.com/oauth/done", want: "https://panel.example.com"},
		{name: "a javascript scheme is refused", staged: "javascript:alert(1)", want: ""},
		{name: "a relative value is refused", staged: "/providers/acme", want: ""},
		{name: "an empty value is refused", staged: "", want: ""},
		{name: "a bare host is refused", staged: "gateway.example.com", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewOAuthHandler(nil, tc.baseURL)
			if got := handler.redirectOrigin(tc.staged); got != tc.want {
				t.Fatalf("redirectOrigin(%q) = %q, want %q", tc.staged, got, tc.want)
			}
		})
	}
}

// TestOAuthHandlerCallbackWithoutAnOriginAnswersJSON covers the deployment with
// no configured base URL: with no origin the flow can vouch for, the callback
// answers the headless shape rather than redirecting somewhere it cannot vouch
// for.
func TestOAuthHandlerCallbackWithoutAnOriginAnswersJSON(t *testing.T) {
	fixture := newOAuthFixture(t, "", oauthProvider("acme"))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/acme/oauth/callback?state=gone", nil)
	req.SetPathValue("provider_id", "acme")
	fixture.handler.Callback(rr, req)
	if got := rr.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want no redirect", got)
	}
	mustErrorCode(t, rr, "VALIDATION_ERROR")
}
