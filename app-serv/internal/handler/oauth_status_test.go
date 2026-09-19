// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_status_test.go
// @for       HTTP tests for the §7.4 OAuth status and refresh routes.
// @uses      context, net/http, net/http/httptest, strings, testing, time.
// @reason    These two routes are the panel's view of credential health and the
//
//	operator's manual override of the worker, so the tables pin that the
//	status answer never carries token material, that the derived refresh
//	state reaches the wire, and that a refresh actually rotates the
//	stored token. AGENTS.md §2.1 requires a happy and a validation path
//	per route; both are here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOAuthHandlerStatus(t *testing.T) {
	cases := []struct {
		name       string
		providerID string
		seed       func(t *testing.T, fixture oauthFixture)
		wantStatus int
		wantFlow   string
		wantStates []string
	}{
		{
			name: "a due token is reported as due", providerID: "acme",
			seed: func(t *testing.T, fixture oauthFixture) {
				seedOAuthAccount(t, fixture, "ep_due", "acme", "due@example.com", time.Now().Add(time.Minute))
			},
			wantStatus: http.StatusOK, wantFlow: "code", wantStates: []string{"due"},
		},
		{
			name: "a fresh token is reported as fresh", providerID: "acme",
			seed: func(t *testing.T, fixture oauthFixture) {
				seedOAuthAccount(t, fixture, "ep_fresh", "acme", "fresh@example.com", time.Now().Add(6*time.Hour))
			},
			wantStatus: http.StatusOK, wantFlow: "code", wantStates: []string{"fresh"},
		},
		{
			name: "a token with no known expiry is reported as missing", providerID: "acme",
			seed: func(t *testing.T, fixture oauthFixture) {
				seedOAuthAccount(t, fixture, "ep_none", "acme", "none@example.com", time.Now())
				endpoint, err := fixture.store.GetByID(context.Background(), "ep_none")
				if err != nil {
					t.Fatalf("reloading: %v", err)
				}
				credential := *endpoint.OAuth()
				credential.ExpiresAt = nil
				endpoint.SetOAuth(&credential, oauthNow)
				if err := fixture.store.Update(context.Background(), endpoint); err != nil {
					t.Fatalf("clearing the expiry: %v", err)
				}
			},
			wantStatus: http.StatusOK, wantFlow: "code", wantStates: []string{"missing"},
		},
		{
			name: "a provider with no accounts reports an empty list", providerID: "acme",
			wantStatus: http.StatusOK, wantFlow: "code", wantStates: []string{},
		},
		{
			name: "an unknown provider is a client mistake", providerID: "nope",
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFixture(t, "", oauthProvider("acme"))
			if tc.seed != nil {
				tc.seed(t, fixture)
			}
			rr := doOAuth(t, http.MethodGet, "/api/v1/providers/"+tc.providerID+"/oauth/status",
				"", tc.providerID, "", fixture.handler.Status)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				mustErrorCode(t, rr, "VALIDATION_ERROR")
				return
			}
			body := decodeBody(t, rr)
			if flow, _ := body["flow"].(string); flow != tc.wantFlow {
				t.Fatalf("flow = %q, want %q", flow, tc.wantFlow)
			}
			rows, _ := body["endpoints"].([]any)
			if len(rows) != len(tc.wantStates) {
				t.Fatalf("endpoints = %d rows, want %d", len(rows), len(tc.wantStates))
			}
			for i, want := range tc.wantStates {
				row, _ := rows[i].(map[string]any)
				if state, _ := row["refresh_state"].(string); state != want {
					t.Fatalf("row %d refresh_state = %q, want %q", i, state, want)
				}
			}
			for _, forbidden := range []string{"stored-access", "stored-refresh", "access_token", "refresh_token"} {
				if strings.Contains(rr.Body.String(), forbidden) {
					t.Fatalf("the status answer carries token material: %s", rr.Body.String())
				}
			}
		})
	}
}

func TestOAuthHandlerRefresh(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCount  float64
	}{
		{name: "an empty body refreshes every due account", body: "",
			wantStatus: http.StatusOK, wantCount: 1},
		{name: "naming the account refreshes it", body: `{"endpoint_id":"ep_due"}`,
			wantStatus: http.StatusOK, wantCount: 1},
		{name: "an unknown account is a not-found", body: `{"endpoint_id":"ep_missing"}`,
			wantStatus: http.StatusNotFound},
		{name: "an over-long endpoint id is refused", body: `{"endpoint_id":"` + strings.Repeat("x", 65) + `"}`,
			wantStatus: http.StatusBadRequest},
		{name: "an unknown field is refused", body: `{"endpoints":[]}`,
			wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOAuthFixture(t, "", oauthProvider("acme"))
			seedOAuthAccount(t, fixture, "ep_due", "acme", "due@example.com", time.Now().Add(time.Minute))
			rr := doOAuth(t, http.MethodPost, "/api/v1/providers/acme/oauth/refresh",
				tc.body, "acme", "", fixture.handler.Refresh)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			body := decodeBody(t, rr)
			if got, _ := body["refreshed"].(float64); got != tc.wantCount {
				t.Fatalf("refreshed = %v, want %v", got, tc.wantCount)
			}
			endpoint, err := fixture.store.GetByID(context.Background(), "ep_due")
			if err != nil {
				t.Fatalf("reloading: %v", err)
			}
			if endpoint.OAuth().LastRefreshAt == nil {
				t.Fatal("the refreshed account must record when it was refreshed")
			}
			if got, _ := body["endpoint_ids"].([]any); len(got) != int(tc.wantCount) {
				t.Fatalf("endpoint_ids = %v, want %d entries", got, int(tc.wantCount))
			}
		})
	}
}
