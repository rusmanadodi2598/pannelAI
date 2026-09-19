// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_callback_test.go
// @for       HTTP tests for the §7.4 OAuth callback, both audiences.
// @uses      internal/domain, internal/service, net/http, net/url, strings,
//
//	testing.
//
// @reason    The callback is the one public route of §7.4 and the only one with
//
//	two answers, so the table pins both: a browser gets a 302 back to
//	the provider page carrying the outcome, a headless caller gets the
//	JSON body it asked for. It also pins that no answer, in either
//	shape, carries token material. The redirect-origin guard is in
//	oauth_callback_guard_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

func TestOAuthHandlerCallbackAnswersHeadlessCallers(t *testing.T) {
	cases := []struct {
		name       string
		state      string
		code       string
		accept     string
		refuse     bool
		wantStatus int
		wantCode   string
		wantCreate bool
	}{
		{
			name: "a first connect answers with the account and a hint",
			code: "the-code", accept: "application/json",
			wantStatus: http.StatusOK, wantCreate: true,
		},
		{
			name: "an unknown state is refused", state: "never-staged", code: "c",
			accept: "application/json", wantStatus: http.StatusBadRequest,
			wantCode: "VALIDATION_ERROR",
		},
		{
			name: "a missing code is refused", code: "",
			accept: "application/json", wantStatus: http.StatusBadRequest,
			wantCode: "VALIDATION_ERROR",
		},
		{
			name: "a token endpoint refusal is an upstream error", code: "bad-code", refuse: true,
			accept: "application/json", wantStatus: http.StatusBadGateway,
			wantCode: "UPSTREAM_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFixture(t, "https://gateway.example.com", oauthProvider("acme"))
			fixture.tokens.infoFn = func() (service.OAuthIdentity, error) {
				return service.OAuthIdentity{Sub: "sub-1", Email: "dev@example.com"}, nil
			}
			if tc.refuse {
				fixture.tokens.grantFn = func(service.TokenGrant) (service.TokenResponse, error) {
					return service.TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant")
				}
			}
			state := tc.state
			if state == "" {
				state = startOAuthFlow(t, fixture, "acme", "")
			}
			rr := doOAuth(t, http.MethodGet,
				"/api/v1/providers/acme/oauth/callback?code="+tc.code+"&state="+url.QueryEscape(state),
				"", "acme", tc.accept, fixture.handler.Callback)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantCode != "" {
				mustErrorCode(t, rr, tc.wantCode)
				return
			}
			body := decodeBody(t, rr)
			if created, _ := body["created"].(bool); created != tc.wantCreate {
				t.Fatalf("created = %v, want %v", created, tc.wantCreate)
			}
			if id, _ := body["endpoint_id"].(string); !strings.HasPrefix(id, "ep_") {
				t.Fatalf("endpoint_id = %q, want an upstream endpoint id", id)
			}
			if email, _ := body["account_email"].(string); email != "dev@example.com" {
				t.Fatalf("account_email = %q", email)
			}
			for _, forbidden := range []string{"at-issued", "rt-issued", "access_token", "refresh_token"} {
				if strings.Contains(rr.Body.String(), forbidden) {
					t.Fatalf("the answer carries token material: %s", rr.Body.String())
				}
			}
		})
	}
}

func TestOAuthHandlerCallbackRedirectsBrowsers(t *testing.T) {
	cases := []struct {
		name       string
		baseURL    string
		redirect   string
		stateIsBad bool
		refuse     bool
		wantQuery  string
	}{
		{
			name:    "a completed flow lands on the provider page",
			baseURL: "https://gateway.example.com", wantQuery: "oauth=connected",
		},
		{
			name:    "a refused state lands with the reason",
			baseURL: "https://gateway.example.com", stateIsBad: true,
			wantQuery: "oauth=error",
		},
		{
			name:    "a provider refusal lands with the reason",
			baseURL: "https://gateway.example.com", refuse: true,
			wantQuery: "oauth=error",
		},
		{
			name:     "without a base URL the staged redirect origin is the fallback",
			redirect: "https://panel.example.com/oauth/done", wantQuery: "oauth=connected",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFixture(t, tc.baseURL, oauthProvider("acme"))
			fixture.tokens.infoFn = func() (service.OAuthIdentity, error) {
				return service.OAuthIdentity{Sub: "sub-1", Email: "dev@example.com"}, nil
			}
			if tc.refuse {
				fixture.tokens.grantFn = func(service.TokenGrant) (service.TokenResponse, error) {
					return service.TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant")
				}
			}
			state := startOAuthFlow(t, fixture, "acme", tc.redirect)
			code := "the-code"
			if tc.stateIsBad {
				state, code = "never-staged", "c"
			}
			rr := doOAuth(t, http.MethodGet,
				"/api/v1/providers/acme/oauth/callback?code="+code+"&state="+url.QueryEscape(state),
				"", "acme", "text/html", fixture.handler.Callback)
			if rr.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302 (body: %s)", rr.Code, rr.Body.String())
			}
			location := rr.Header().Get("Location")
			wantOrigin := tc.baseURL
			if wantOrigin == "" {
				wantOrigin = "https://panel.example.com"
			}
			if !strings.HasPrefix(location, wantOrigin+"/providers/acme?") {
				t.Fatalf("Location = %q, want the provider page under %s", location, wantOrigin)
			}
			if !strings.Contains(location, tc.wantQuery) {
				t.Fatalf("Location = %q, want %s", location, tc.wantQuery)
			}
			if tc.wantQuery == "oauth=connected" {
				if !strings.Contains(location, "endpoint_id=ep_") {
					t.Fatalf("Location = %q, want the endpoint the flow wrote", location)
				}
				return
			}
			parsed, err := url.Parse(location)
			if err != nil {
				t.Fatalf("parsing Location: %v", err)
			}
			if reason := parsed.Query().Get("oauth_error"); reason == "" {
				t.Fatal("a failed callback must carry the reason for the panel's copy")
			}
		})
	}
}
