// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/oauth_grant_call_test.go
// @for       That the token-endpoint call is built from the neutral grant shape and its typed answer decoded as the wire carries it.
// @uses      context, net/http, net/http/httptest, testing, internal/domain.
// @reason    R19 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: the grant seam spoke *http.Request, an HTTP-layer type, inside the service layer (AGENTS.md §1.5). The seam now carries method, URL, headers, and body, and this test proves the wire request built from that shape is what a provider actually receives.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package oauthhttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDoGrant_SendsTheNeutralCall pins the transport half of the token seam: the
// method, URL, headers, and body the neutral grant call carries are what leave,
// and the typed answer is what comes back.
func TestDoGrant_SendsTheNeutralCall(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotType   string
		gotBody   []byte
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-1","refresh_token":"rt-1","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	call := grantCall{
		method: http.MethodPost,
		url:    server.URL + "/oauth/token",
		headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		body: `{"grant_type":"authorization_code","code":"c-1"}`,
	}
	answer, err := doGrant(context.Background(), server.Client(), call)
	if err != nil {
		t.Fatalf("doGrant() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/oauth/token" {
		t.Fatalf("path = %q, want /oauth/token", gotPath)
	}
	if gotType != "application/json" {
		t.Fatalf("content type = %q, want application/json", gotType)
	}
	if string(gotBody) != call.body {
		t.Fatalf("body = %q, want the grant call's own body", gotBody)
	}
	if answer.AccessToken != "at-1" || answer.ExpiresIn != 3600 {
		t.Fatalf("answer = %+v, want the decoded token response", answer)
	}
}
