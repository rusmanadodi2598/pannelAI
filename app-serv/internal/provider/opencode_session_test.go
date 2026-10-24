// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session_test.go
// @for       The session the OpenCode free tier presents and serializes on, and the user-agent rule.
// @uses      testing, net/http, internal/registry.
// @reason    The upstream accounts free-tier quota per session, so the header ApplyAuth mints and the key the transport serializes on must be the same value, stable per endpoint and absent on the keyed lanes. The user agent is the identity gate: a v2 CLI form is kept, every v1 form is replaced.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package provider

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestOpenCode_SessionKeyMatchesTheSessionItPresents pins the serialization key
// to the header: the key the transport serializes on must be the session
// ApplyAuth mints, or two requests presenting one session could run at once (or
// two unrelated requests could serialize against each other). The key is a pure
// function of the entry's client identity and the endpoint's id, and the header
// is minted from the same pair.
func TestOpenCode_SessionKeyMatchesTheSessionItPresents(t *testing.T) {
	entry := opencodeEntry("https://opencode.ai", "openai")
	connector := NewOpenCode(entry)
	var serializer SessionSerializer = connector

	cases := []struct {
		name       string
		endpointID string
	}{
		{name: "a stored endpoint", endpointID: "ep-1"},
		{name: "the synthesized free-lane endpoint", endpointID: "virtual:opencode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presented := requestWithAuth(t, connector, tc.endpointID)
			key := serializer.SessionKey(Request{Provider: entry, Credential: NoCredential(tc.endpointID)})
			if key == "" {
				t.Fatal("SessionKey() = empty, want the session the request presents")
			}
			if want := presented.Header.Get("x-opencode-session"); key != want {
				t.Fatalf("SessionKey() = %q, want the presented session %q", key, want)
			}
		})
	}

	// Two endpoints must not share one serialization key, exactly as they must
	// not share one session.
	first := serializer.SessionKey(Request{Provider: entry, Credential: NoCredential("ep-a")})
	second := serializer.SessionKey(Request{Provider: entry, Credential: NoCredential("ep-b")})
	if first == second {
		t.Fatal("two endpoints share one session key, want distinct keys")
	}
}

// TestOpenCode_SessionKeyIsEmptyOnAKeyedLane pins the other side of the rule:
// the PAYG and subscription lanes present a credential and no session headers,
// so naming a session for them would serialize traffic the upstream accounts by
// key. The key is empty exactly when ApplyAuth skips the identity headers.
func TestOpenCode_SessionKeyIsEmptyOnAKeyedLane(t *testing.T) {
	keyed := opencodeEntry("https://opencode.ai", "openai")
	keyed.ID = "opencode-go"
	keyed.NoAuth = false
	keyed.AuthType = registry.AuthAPIKey
	keyed.Transport.NoAuth = false
	keyed.Transport.Auth = registry.AuthConfig{Header: "Authorization", Scheme: "bearer"}
	connector := NewOpenCode(keyed)

	request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/inference/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	if err := connector.ApplyAuth(request, StaticKey("ep-1", "key-1", "sk-secret")); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	if got := request.Header.Get("x-opencode-session"); got != "" {
		t.Fatalf("x-opencode-session = %q, want no session on a keyed lane", got)
	}
	if got := connector.SessionKey(Request{Provider: keyed, Credential: StaticKey("ep-1", "key-1", "sk-secret")}); got != "" {
		t.Fatalf("SessionKey() = %q, want empty on a lane that presents no session", got)
	}

	// A keyed entry reached without credential material falls through to the
	// public path, which does present a session, so the key must be named.
	if got := connector.SessionKey(Request{Provider: keyed, Credential: NoCredential("ep-1")}); got == "" {
		t.Fatal("SessionKey() = empty, want the session the credential-free fall-through presents")
	}
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
