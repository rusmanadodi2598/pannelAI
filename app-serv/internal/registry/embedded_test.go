// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/embedded_test.go
// @for       Tests that the committed embedded document decodes into a complete provider index.
// @uses      testing, strings, internal/registry.
// @reason    SPEC-API-001 §6 makes the registry the source of truth for every
//
//	provider, so the ported document must decode into the shapes the
//	reference carried, with no live third-party credential and no
//	provider silently dropped on load.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import (
	"strings"
	"testing"
)

func TestLoad_EmbeddedRegistryDecodes(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if idx.Count() != 34 {
		t.Fatalf("Count() = %d, want the owner's KEEP set of 34 providers", idx.Count())
	}
	if idx.Revision() == "" {
		t.Fatal("Revision() must name the reference revision the document came from")
	}

	// A representative provider from each shape the port has to carry: an
	// OpenAI-format key provider, a Claude-format one, two OAuth providers (one
	// declaring its client credentials on the transport rather than in an oauth
	// block), a provider on a non-chat wire format, and a credential-free one.
	//
	// auth_type is the default mode, derived at load because the reference leaves
	// it unset for most entries: no_auth wins, then oauth when the provider has a
	// flow, otherwise api_key.
	cases := []struct {
		id       string
		format   string
		auth     string
		hasOAuth bool
	}{
		{id: "deepseek", format: "openai", auth: AuthAPIKey},
		{id: "minimax", format: "claude", auth: AuthAPIKey},
		{id: "grok-cli", format: "openai-responses", auth: AuthOAuth, hasOAuth: true},
		{id: "xai", format: "openai", auth: AuthOAuth, hasOAuth: true},
		{id: "commandcode", format: "commandcode", auth: AuthAPIKey},
		{id: "opencode", format: "openai", auth: AuthNone},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			p, ok := idx.Provider(tc.id)
			if !ok {
				t.Fatalf("provider %q is missing from the embedded registry", tc.id)
			}
			if p.Transport.Format != tc.format {
				t.Fatalf("format = %q, want %q", p.Transport.Format, tc.format)
			}
			if p.AuthType != tc.auth {
				t.Fatalf("auth_type = %q, want %q", p.AuthType, tc.auth)
			}
			if tc.hasOAuth && p.OAuth == nil {
				t.Fatal("oauth block must be present for an OAuth provider")
			}
			if !tc.hasOAuth && p.OAuth != nil {
				t.Fatalf("oauth block is present for %q, which declares no flow", tc.id)
			}
		})
	}

	// xai accepts both a flow and a plain key, so its default stays oauth while
	// both remain declared as modes.
	xai, ok := idx.Provider("xai")
	if !ok {
		t.Fatal("xai must be present in the embedded registry")
	}
	if len(xai.AuthModes) != 2 {
		t.Fatalf("xai auth_modes = %v, want both oauth and apikey", xai.AuthModes)
	}

	// The client secret must never have been ported: the reference checkout holds
	// live third-party credentials and this document is committed.
	for _, p := range idx.All() {
		if p.OAuth != nil && strings.Contains(strings.ToLower(p.OAuth.ClientSecret), "gocspx") {
			t.Fatalf("provider %s carries a live client secret", p.ID)
		}
	}
}
