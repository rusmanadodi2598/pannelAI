// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/provider_tokenrouter_test.go
// @for       The TokenRouter entry and the credential placement of every video block.
// @uses      testing, internal/registry.
// @reason    TokenRouter is the one kept provider the reference declares no video block for, and its transport's thinking format is what the reasoning port must honour, so both facts are pinned here rather than left to a live request. The bearer placement is pinned for every video block because app-serv sends no credential at all when a block declares none, which would turn an authenticated upstream into a 401.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-12
package registry

import "testing"

// TestRegistry_TokenRouterEntry pins the transport, the model set, and the
// media blocks the provider is served with.
func TestRegistry_TokenRouterEntry(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	entry, ok := idx.Provider("tokenrouter")
	if !ok {
		t.Fatal("tokenrouter is missing from the embedded registry")
	}
	if entry.AuthType != AuthAPIKey {
		t.Fatalf("auth_type = %q, want %q", entry.AuthType, AuthAPIKey)
	}
	if !entry.PassthroughModels {
		t.Fatal("passthrough_models must be set: the provider accepts ids beyond its seed list")
	}
	if entry.Transport.Format != "openai" {
		t.Fatalf("format = %q, want openai", entry.Transport.Format)
	}
	if entry.Transport.BaseURL != "https://api.tokenrouter.com/v1/chat/completions" {
		t.Fatalf("base_url = %q", entry.Transport.BaseURL)
	}
	if entry.Transport.ValidateURL != "https://api.tokenrouter.com/v1/models" {
		t.Fatalf("validate_url = %q, want the models endpoint the provider validates a key against", entry.Transport.ValidateURL)
	}
	if entry.Transport.ThinkingFormat != "tokenrouter" {
		t.Fatalf("thinking_format = %q, want the provider's own reasoning shape", entry.Transport.ThinkingFormat)
	}
	if len(entry.Models) == 0 {
		t.Fatal("the seed model list must not be empty")
	}
	for _, kind := range []MediaKind{MediaEmbedding, MediaImage, MediaVideo} {
		if !entry.Media.Supports(kind) {
			t.Fatalf("media block %q is missing", kind)
		}
	}
	video, _ := entry.Media.For(MediaVideo)
	if video.BaseURL != "https://api.tokenrouter.com/v1/video/generations" {
		t.Fatalf("video base_url = %q, want the provider's create endpoint", video.BaseURL)
	}
}

// TestRegistry_VideoBlocksAuthenticate pins that every video block places the
// credential as a bearer. The reference's video proxy always sends one
// (open-sse/handlers/videoCore.js:39-45) while its videoConfig blocks declare no
// auth fields, so a block copied verbatim would reach the upstream anonymous.
func TestRegistry_VideoBlocksAuthenticate(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	seen := 0
	for _, provider := range idx.All() {
		video, ok := provider.Media.For(MediaVideo)
		if !ok {
			continue
		}
		seen++
		// The media block's auth_type is the reference's own spelling rather
		// than the provider-level AuthAPIKey constant, so the assertion is that
		// a placement is declared at all plus the scheme it names.
		if video.AuthType == "" {
			t.Fatalf("%s video block declares no auth_type, so the call would be sent anonymous", provider.ID)
		}
		if video.AuthHeader != "bearer" {
			t.Fatalf("%s video auth_header = %q, want bearer", provider.ID, video.AuthHeader)
		}
	}
	if seen == 0 {
		t.Fatal("no provider declares a video block, so this test proved nothing")
	}
}
