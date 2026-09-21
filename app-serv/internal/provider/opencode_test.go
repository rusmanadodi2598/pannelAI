// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_test.go
// @for       The OpenCode Free connector: its per-model URL, its request shape,
//
//	and the identity headers the free tier gates on.
//
// @uses      testing, net/http, encoding/json, internal/registry.
// @reason    The OpenCode free tier answers 403 to a request that does not carry
//
//	all of three things at once (a streamed body, the bash and read
//	decoys, and the CLI's own identity headers), and 502 to a request
//	sent to the host root. Both are upstream rules, not gateway
//	choices, so they belong to the connector and are pinned here rather
//	than left to a live probe.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// opencodeEntry builds the registry entry the connector is bound to, with the
// declared base and transport format a test varies.
func opencodeEntry(baseURL, format string) registry.Provider {
	return registry.Provider{
		ID:     "opencode",
		NoAuth: true,
		Transport: registry.Transport{
			BaseURL: baseURL,
			Format:  format,
			NoAuth:  true,
			Headers: map[string]string{"x-opencode-client": "desktop"},
		},
	}
}

// TestOpenCode_EndpointRoutesPerModelWire pins the URL rule the connector exists
// to apply: the model's own wire decides the path, and every base shape an
// operator can store (a bare host, the documented /zen/v1 base, a full path, and
// a trailing slash) composes into one correct URL rather than a doubled segment
// or a request sent to the host root.
func TestOpenCode_EndpointRoutesPerModelWire(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		format  string
		model   registry.Model
		wantURL string
	}{
		{
			name: "a bare host routes a Responses model to /zen/v1/responses",
			base: "https://opencode.ai", format: "openai",
			model:   registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
			wantURL: "https://opencode.ai/zen/v1/responses",
		},
		{
			name: "a bare host routes a chat model to /zen/v1/chat/completions",
			base: "https://opencode.ai", format: "openai",
			model:   registry.Model{ID: "big-pickle"},
			wantURL: "https://opencode.ai/zen/v1/chat/completions",
		},
		{
			name: "the documented /zen/v1 base is not doubled",
			base: "https://opencode.ai/zen/v1", format: "openai",
			model:   registry.Model{ID: "muse-spark-1.2-contributor-free", TargetFormat: "openai-responses"},
			wantURL: "https://opencode.ai/zen/v1/responses",
		},
		{
			name: "a trailing slash on the base is not doubled",
			base: "https://opencode.ai/zen/v1/", format: "openai",
			model:   registry.Model{ID: "big-pickle"},
			wantURL: "https://opencode.ai/zen/v1/chat/completions",
		},
		{
			name: "a base that already names the leaf is used as written",
			base: "https://opencode.ai/zen/v1/responses", format: "openai",
			model:   registry.Model{ID: "muse-spark-1.3-contributor-free", TargetFormat: "openai-responses"},
			wantURL: "https://opencode.ai/zen/v1/responses",
		},
		{
			name: "a Claude-wire model routes to /zen/v1/messages",
			base: "https://opencode.ai", format: "openai",
			model:   registry.Model{ID: "union-alpha", TargetFormat: "claude"},
			wantURL: "https://opencode.ai/zen/v1/messages",
		},
		{
			name: "the provider's own format decides when the model declares none",
			base: "https://opencode.ai", format: "openai-responses",
			model:   registry.Model{ID: "an-undocumented-model"},
			wantURL: "https://opencode.ai/zen/v1/responses",
		},
		{
			name: "a host with a port keeps it",
			base: "http://127.0.0.1:8898", format: "openai",
			model:   registry.Model{ID: "big-pickle"},
			wantURL: "http://127.0.0.1:8898/zen/v1/chat/completions",
		},
		{
			name: "a base with a non-zen path keeps that path",
			base: "https://gateway.example.test/opencode", format: "openai",
			model:   registry.Model{ID: "big-pickle"},
			wantURL: "https://gateway.example.test/opencode/zen/v1/chat/completions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			connector := NewOpenCode(opencodeEntry(tc.base, tc.format))
			got, err := connector.Endpoint(Request{Model: tc.model, Provider: opencodeEntry(tc.base, tc.format)}, NoCredential("ep-1"))
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			if got != tc.wantURL {
				t.Fatalf("Endpoint() = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

// TestOpenCode_EndpointRefusesAnEmptyBase pins the refusal: a connector whose
// entry declares no host cannot build a URL, and reporting that is better than
// sending the request to a relative path.
func TestOpenCode_EndpointRefusesAnEmptyBase(t *testing.T) {
	for _, base := range []string{"", "   ", "not-a-url", "/zen/v1"} {
		t.Run(base, func(t *testing.T) {
			connector := NewOpenCode(opencodeEntry(base, "openai"))
			if _, err := connector.Endpoint(Request{Model: registry.Model{ID: "m"}}, NoCredential("ep")); err == nil {
				t.Fatalf("Endpoint() with base %q = nil error, want a refusal", base)
			}
		})
	}
}
