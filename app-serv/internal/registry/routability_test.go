// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/routability_test.go
// @for       Tests for the format rule that decides whether a provider's chat
//
//	path can be served, and for the media configs that carry their own
//	credential placement.
//
// @uses      testing, internal/registry.
// @reason    A provider can be listed and configured yet still not answerable,
//
//	because P1 translates only the OpenAI, Anthropic, and OpenAI
//	Responses formats. That gap has to be a value the caller reads, not
//	a failure discovered under traffic; and a media service often
//	authenticates differently from its provider's chat transport, so
//	both rules are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

func TestProvider_ChatRoutability(t *testing.T) {
	cases := []struct {
		name   string
		format string
		want   string
		wantOK bool
	}{
		{name: "the default format is translated natively", format: "", want: Routable, wantOK: true},
		{name: "openai is translated natively", format: "openai", want: Routable, wantOK: true},
		{name: "claude is translated natively", format: "claude", want: Routable, wantOK: true},
		{name: "openai-responses is translated natively", format: "openai-responses", want: Routable, wantOK: true},
		{name: "a bespoke protocol needs a connector", format: "kiro", want: RoutableNeedsConnector},
		{name: "a cursor-style protocol needs a connector", format: "cursor", want: RoutableNeedsConnector},
		{name: "an antigravity-style protocol needs a connector", format: "antigravity", want: RoutableNeedsConnector},
		{name: "a gemini-style protocol needs a connector", format: "gemini", want: RoutableNeedsConnector},
		{name: "an unknown format needs a connector", format: "something-new", want: RoutableNeedsConnector},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := Provider{ID: "p", Transport: Transport{Format: tc.format}}
			if got := provider.ChatRoutability(); got != tc.want {
				t.Fatalf("ChatRoutability() = %q, want %q", got, tc.want)
			}
			if got := provider.IsChatRoutable(); got != tc.wantOK {
				t.Fatalf("IsChatRoutable() = %v, want %v", got, tc.wantOK)
			}
		})
	}
}

// TestProvider_ChatRoutabilityOnEmbeddedRegistry checks the rule against the
// real document, including the providers the owner listed that P1 cannot serve
// yet. Reporting them as connector-only is deliberate: the alternative is an
// endpoint that always fails.
func TestProvider_ChatRoutabilityOnEmbeddedRegistry(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Providers on the three formats P1 translates. Two are deliberately absent
	// despite being on the owner's list: `commandcode` declares format
	// "commandcode" and `gemini-cli` declares "gemini-cli", both bespoke
	// protocols, so they are in the connector-only bucket below.
	nativelyServed := []string{
		"deepseek", "anthropic", "openai", "openrouter", "nvidia",
		"glm", "glm-cn", "minimax", "minimax-cn", "xiaomi-mimo",
		"kimi", "codex",
	}
	for _, id := range nativelyServed {
		p, ok := idx.Provider(id)
		if !ok {
			t.Fatalf("provider %q is missing from the embedded registry", id)
		}
		if !p.IsChatRoutable() {
			t.Fatalf("provider %q (format %q) must be served natively", id, p.Transport.Format)
		}
	}

	// These speak protocols P1 does not translate, so a connector is required.
	// Four are on the owner's list and would otherwise be "configured but always
	// failing".
	needsConnector := []string{"kiro", "cursor", "antigravity", "gemini", "gemini-cli", "commandcode"}
	for _, id := range needsConnector {
		p, ok := idx.Provider(id)
		if !ok {
			t.Fatalf("provider %q is missing from the embedded registry", id)
		}
		if p.IsChatRoutable() {
			t.Fatalf("provider %q (format %q) must require a connector", id, p.Transport.Format)
		}
	}

	// Every provider must land in one bucket or the other, never neither.
	for _, p := range idx.All() {
		switch p.ChatRoutability() {
		case Routable, RoutableNeedsConnector:
		default:
			t.Fatalf("provider %q has an undefined routability %q", p.ID, p.ChatRoutability())
		}
	}
}

// TestMediaConfigs_CarryTheirOwnAuthPlacement is the reason media blocks are
// ported separately from transport: Gemini reads a query-param key for
// embeddings while its chat transport uses a header, so a caller that reused the
// transport's placement would authenticate incorrectly.
func TestMediaConfigs_CarryTheirOwnAuthPlacement(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	cases := []struct {
		provider    string
		kind        MediaKind
		wantBaseURL string
		wantAuth    string
		wantHeader  string
	}{
		{
			provider: "gemini", kind: MediaEmbedding,
			wantBaseURL: "https://generativelanguage.googleapis.com/v1beta/models",
			wantAuth:    "apikey", wantHeader: "key",
		},
		{
			provider: "openai", kind: MediaEmbedding,
			wantBaseURL: "https://api.openai.com/v1/embeddings",
			wantAuth:    "apikey", wantHeader: "bearer",
		},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"/"+string(tc.kind), func(t *testing.T) {
			p, ok := idx.Provider(tc.provider)
			if !ok {
				t.Fatalf("provider %q is missing", tc.provider)
			}
			config, ok := p.Media.For(tc.kind)
			if !ok {
				t.Fatalf("provider %q declares no %s config", tc.provider, tc.kind)
			}
			if config.BaseURL != tc.wantBaseURL {
				t.Fatalf("base_url = %q, want %q", config.BaseURL, tc.wantBaseURL)
			}
			if config.AuthType != tc.wantAuth {
				t.Fatalf("auth_type = %q, want %q", config.AuthType, tc.wantAuth)
			}
			if config.AuthHeader != tc.wantHeader {
				t.Fatalf("auth_header = %q, want %q", config.AuthHeader, tc.wantHeader)
			}
		})
	}

	// The media blocks must be present broadly enough to build the media
	// screens on, not just for the two providers asserted above.
	withEmbeddings := 0
	for _, p := range idx.All() {
		if p.Media.Supports(MediaEmbedding) {
			withEmbeddings++
		}
	}
	if withEmbeddings < 5 {
		t.Fatalf("only %d providers declare an embedding config, want the ported set", withEmbeddings)
	}
}
