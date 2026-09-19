// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_test.go
// @for       HTTP tests for the §7.4 OAuth start route.
// @uses      net/http, strings, testing.
// @reason    AGENTS.md §2.1 requires a happy path and a validation-failure path
//
//	per route. Start is the route that decides which callback a provider
//	will call, so the table pins the redirect choice, the PKCE challenge,
//	and every refusal that keeps a browser from being sent somewhere the
//	gateway did not choose. The status and refresh routes are in
//	oauth_status_test.go, and the callback in oauth_callback_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"strings"
	"testing"
)

func TestOAuthHandlerStart(t *testing.T) {
	cases := []struct {
		name       string
		baseURL    string
		providerID string
		body       string
		wantStatus int
		wantIn     string
	}{
		{
			name:    "a configured base URL becomes the gateway callback",
			baseURL: "https://gateway.example.com", providerID: "acme",
			body: `{}`, wantStatus: http.StatusOK,
			wantIn: "redirect_uri=https%3A%2F%2Fgateway.example.com%2Fapi%2Fv1%2Fproviders%2Facme%2Foauth%2Fcallback",
		},
		{
			name:    "an explicit redirect URI wins over the base URL",
			baseURL: "https://gateway.example.com", providerID: "acme",
			body: `{"redirect_uri":"https://panel.example.com/callback"}`, wantStatus: http.StatusOK,
			wantIn: "redirect_uri=https%3A%2F%2Fpanel.example.com%2Fcallback",
		},
		{
			name:    "the PKCE challenge rides along for an S256 provider",
			baseURL: "https://gateway.example.com", providerID: "acme",
			body: `{}`, wantStatus: http.StatusOK,
			wantIn: "code_challenge_method=S256",
		},
		{
			name:    "the declared scopes are requested",
			baseURL: "https://gateway.example.com", providerID: "acme",
			body: `{}`, wantStatus: http.StatusOK,
			wantIn: "scope=scope%3Aone+scope%3Atwo",
		},
		{
			name: "a non-http redirect URI is refused", baseURL: "https://gateway.example.com",
			providerID: "acme", body: `{"redirect_uri":"javascript:alert(1)"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "a relative redirect URI is refused", baseURL: "https://gateway.example.com",
			providerID: "acme", body: `{"redirect_uri":"/callback"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "an unknown provider is a client mistake", baseURL: "https://gateway.example.com",
			providerID: "nope", body: `{}`, wantStatus: http.StatusBadRequest,
		},
		{
			name: "no redirect and no base URL cannot build a callback", baseURL: "",
			providerID: "acme", body: `{}`, wantStatus: http.StatusBadRequest,
		},
		{
			name: "an unknown field is refused", baseURL: "https://gateway.example.com",
			providerID: "acme", body: `{"redirect":"https://panel.example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "a malformed body is refused", baseURL: "https://gateway.example.com",
			providerID: "acme", body: `{"redirect_uri":`,
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFixture(t, tc.baseURL, oauthProvider("acme"))
			rr := doOAuth(t, http.MethodPost, "/api/v1/providers/"+tc.providerID+"/oauth/start",
				tc.body, tc.providerID, "", fixture.handler.Start)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				mustErrorCode(t, rr, "VALIDATION_ERROR")
				return
			}
			body := decodeBody(t, rr)
			authorize, _ := body["authorize_url"].(string)
			state, _ := body["state"].(string)
			if !strings.Contains(authorize, tc.wantIn) {
				t.Fatalf("authorize_url %q must contain %q", authorize, tc.wantIn)
			}
			if state == "" {
				t.Fatal("state must not be empty: it is the replay guard")
			}
			if !strings.HasPrefix(authorize, "https://auth.example.com/acme/authorize?") {
				t.Fatalf("authorize_url = %q, want the provider's endpoint", authorize)
			}
		})
	}
}

// TestOAuthHandlerStartMintsADistinctStatePerFlow pins the replay guard's first
// half: two flows never share a state value, so consuming one cannot authorize
// the other.
func TestOAuthHandlerStartMintsADistinctStatePerFlow(t *testing.T) {
	fixture := newOAuthFixture(t, "https://gateway.example.com", oauthProvider("acme"))
	first := startOAuthFlow(t, fixture, "acme", "")
	second := startOAuthFlow(t, fixture, "acme", "")
	if first == second {
		t.Fatal("two flows minted the same state")
	}
	if _, ok := fixture.states.staged[first]; !ok {
		t.Fatal("the first flow's state was not staged")
	}
	if _, ok := fixture.states.staged[second]; !ok {
		t.Fatal("the second flow's state was not staged")
	}
}
