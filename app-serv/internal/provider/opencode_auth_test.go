// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_auth_test.go
// @for       The credential and identity the OpenCode free tier is presented.
// @uses      testing, net/http.
// @reason    The free tier reads a literal public bearer and the v2 CLI's own identity headers, and it accounts quota per session, so both the value and its stability are rules rather than details. The v1 turn header must be absent: the v2 upstream answers any request carrying it with 403 FreeTierError. They are pinned apart from the URL rule so each file stays inside the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
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
			request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/inference/v1/chat/completions", nil)
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
// shape), it must be stable for one endpoint so a caller's quota is not burned
// by a fresh identity per request, and it must appear under all four names the
// v2 CLI presents it with.
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
			request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/inference/v1/chat/completions", nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if err := connector.ApplyAuth(request, NoCredential(tc.endpointID)); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}

			if got := request.Header.Get("User-Agent"); got != OpenCodeUserAgent {
				t.Fatalf("User-Agent = %q, want the v2 CLI identity %q", got, OpenCodeUserAgent)
			}
			if got := request.Header.Get("x-opencode-client"); got != "cli" {
				t.Fatalf("x-opencode-client = %q, want cli", got)
			}
			if got := request.Header.Get("x-opencode-project"); got != "global" {
				t.Fatalf("x-opencode-project = %q, want global", got)
			}
			session := request.Header.Get("x-opencode-session")
			if !OpenCodeSessionRE.MatchString(session) {
				t.Fatalf("x-opencode-session = %q, want the canonical shape", session)
			}
			for _, header := range []string{"x-opencode-session-id", "x-session-affinity", "x-session-id"} {
				if got := request.Header.Get(header); got != session {
					t.Fatalf("%s = %q, want the session %q", header, got, session)
				}
			}
			if got := request.Header.Get("x-opencode-request"); got != "" {
				t.Fatalf("x-opencode-request = %q, want the v1 turn header never sent", got)
			}

			// The same endpoint must present the same session on the next call,
			// because the upstream accounts free-tier quota per session.
			second, err := http.NewRequest(http.MethodPost, "https://opencode.ai/inference/v1/chat/completions", nil)
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
	request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/inference/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	if err := connector.ApplyAuth(request, NoCredential(endpointID)); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	return request
}

// TestOpenCode_ApplyAuthChoosesTheUserAgent pins the v2 identity rule: a
// downstream agent that already presents the v2 CLI form keeps its own agent
// string, and every other agent, including the v1 form the upstream now refuses
// with 403, is replaced with the connector's own.
func TestOpenCode_ApplyAuthChoosesTheUserAgent(t *testing.T) {
	connector := NewOpenCode(opencodeEntry("https://opencode.ai", "openai"))

	cases := []struct {
		name string
		ua   string
		want string
	}{
		{name: "no agent", ua: "", want: OpenCodeUserAgent},
		{name: "a browser agent", ua: "Claude-Code/1.0", want: OpenCodeUserAgent},
		{name: "the bare v1 agent", ua: "opencode", want: OpenCodeUserAgent},
		{name: "a v1 agent with a version", ua: "opencode/1.19.0", want: OpenCodeUserAgent},
		{name: "a v1 agent with suffixes", ua: "opencode/1.18.31 ai-sdk/provider-utils/4.0.40 runtime/bun/1.3.14", want: OpenCodeUserAgent},
		{name: "a browser string carrying an incidental token", ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) opencode/invalid", want: OpenCodeUserAgent},
		{name: "a v2 CLI agent", ua: "opencode/latest/2.0.29/cli", want: "opencode/latest/2.0.29/cli"},
		{name: "a v2 desktop agent", ua: "opencode/stable/2.1.0/desktop", want: "opencode/stable/2.1.0/desktop"},
		{name: "a v1-major v2-shaped agent", ua: "opencode/latest/1.99.0/cli", want: OpenCodeUserAgent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/inference/v1/chat/completions", nil)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if tc.ua != "" {
				request.Header.Set("User-Agent", tc.ua)
			}
			if err := connector.ApplyAuth(request, NoCredential("ep-1")); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if got := request.Header.Get("User-Agent"); got != tc.want {
				t.Fatalf("User-Agent = %q, want %q", got, tc.want)
			}
		})
	}
}
