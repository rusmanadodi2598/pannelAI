// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_auth_test.go
// @for       The credential and identity the OpenCode free tier is presented.
// @uses      testing, net/http.
// @reason    The free tier reads a literal public bearer and the CLI's own
//
//	identity headers, and it accounts quota per session, so both the value
//	and its stability are rules rather than details. They are pinned apart
//	from the URL rule so each file stays inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"net/http"
	"testing"
)

// TestOpenCode_ApplyAuthSendsThePublicBearer pins the free tier's credential:
// OpenCode Free reads `Authorization: Bearer public`, so a keyless account still
// presents one, and a configured key is never sent to an endpoint that pools
// anonymous traffic.
func TestOpenCode_ApplyAuthSendsThePublicBearer(t *testing.T) {
	cases := []struct {
		name string
		cred Credential
	}{
		{name: "no credential material", cred: NoCredential("ep-1")},
		{name: "an unrelated static key", cred: StaticKey("ep-1", "key-1", "sk-secret")},
		{name: "an unrelated oauth token", cred: OAuthToken("ep-1", "key-1", "ya29.token")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))
			if err := connector.ApplyAuth(request, tc.cred); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if got := request.Header.Get("Authorization"); got != "Bearer public" {
				t.Fatalf("Authorization = %q, want the public bearer", got)
			}
		})
	}
}

// TestOpenCode_ApplyAuthSetsTheIdentityHeaders pins the headers the free tier
// gates on. The session must be canonical (the upstream refuses any other
// shape), and it must be stable for one endpoint so a caller's quota is not
// burned by a fresh identity per request.
func TestOpenCode_ApplyAuthSetsTheIdentityHeaders(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name       string
		endpointID string
	}{
		{name: "an endpoint id", endpointID: "ep-1"},
		{name: "another endpoint id", endpointID: "ep-2"},
		{name: "a uuid-shaped id", endpointID: "0f8fad5b-d9cb-469f-a165-70867728950e"},
		{name: "an empty id", endpointID: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if err := connector.ApplyAuth(request, NoCredential(tc.endpointID)); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}

			userAgent := request.Header.Get("User-Agent")
			if userAgent == "" || userAgent == "Go-http-client/1.1" {
				t.Fatalf("User-Agent = %q, want the CLI identity the free tier checks", userAgent)
			}
			if got := request.Header.Get("x-opencode-client"); got != "desktop" {
				t.Fatalf("x-opencode-client = %q, want desktop", got)
			}
			if got := request.Header.Get("x-opencode-project"); got != "global" {
				t.Fatalf("x-opencode-project = %q, want global", got)
			}
			if session := request.Header.Get("x-opencode-session"); !OpenCodeSessionRE.MatchString(session) {
				t.Fatalf("x-opencode-session = %q, want the canonical shape", session)
			}

			// The same endpoint must present the same session on the next call,
			// because the upstream accounts free-tier quota per session.
			second, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if err := connector.ApplyAuth(second, NoCredential(tc.endpointID)); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if a, b := request.Header.Get("x-opencode-session"), second.Header.Get("x-opencode-session"); a != b {
				t.Fatalf("session = %q then %q, want one stable identity per endpoint", a, b)
			}
		})
	}

	// Two endpoints must not share one identity, or one caller's quota would be
	// spent by another.
	first := requestWithAuth(t, connector, "ep-a")
	second := requestWithAuth(t, connector, "ep-b")
	if first.Header.Get("x-opencode-session") == second.Header.Get("x-opencode-session") {
		t.Fatal("two endpoints share one session, want distinct identities")
	}
}

// requestWithAuth builds a request the connector has authenticated, so a test
// reads the headers it produced.
func requestWithAuth(t *testing.T, connector *OpenCode, endpointID string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	if err := connector.ApplyAuth(request, NoCredential(endpointID)); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	return request
}
