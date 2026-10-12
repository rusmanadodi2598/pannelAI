// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/provider_vertex_test.go
// @for       The Vertex AI entry: its video-only catalog, the service-account auth declaration, and the credential placement of its video block.
// @uses      testing, internal/registry.
// @reason    Vertex is the one kept provider whose credential is neither a plain key nor an OAuth flow, and whose chat models the port deliberately does not list, so both facts are pinned here rather than left to a live request.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-12
package registry

import "testing"

// TestRegistry_VertexEntry pins the entry the owner's directive added
// (docs/PORT/012): the transport, the service-account declaration, and the
// routability answer that keeps its chat path out of the catalog.
func TestRegistry_VertexEntry(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	entry, ok := idx.Provider("vertex")
	if !ok {
		t.Fatal("vertex is missing from the embedded registry")
	}
	if entry.AuthType != AuthServiceAccount {
		t.Fatalf("auth_type = %q, want %q", entry.AuthType, AuthServiceAccount)
	}
	if len(entry.AuthModes) != 1 || entry.AuthModes[0] != AuthServiceAccount {
		t.Fatalf("auth_modes = %v, want only %q", entry.AuthModes, AuthServiceAccount)
	}
	if entry.Transport.Format != "vertex" {
		t.Fatalf("format = %q, want vertex", entry.Transport.Format)
	}
	if entry.Transport.BaseURL != "https://aiplatform.googleapis.com" {
		t.Fatalf("base_url = %q", entry.Transport.BaseURL)
	}
	// The chat wire has no translator, so the entry must report that rather
	// than offer a chat route that always fails.
	if entry.ChatRoutability() != RoutableNeedsConnector {
		t.Fatalf("ChatRoutability() = %q, want %q for the vertex wire", entry.ChatRoutability(), RoutableNeedsConnector)
	}
	if len(entry.ServiceKinds) != 1 || entry.ServiceKinds[0] != "video" {
		t.Fatalf("service_kinds = %v, want only video", entry.ServiceKinds)
	}
}

// TestRegistry_VertexCatalogIsVideoOnly pins the model filter. The reference
// declares four Gemini chat models beside the four Veo models, and a catalog
// that offered the chat half would advertise requests this router can never
// serve.
func TestRegistry_VertexCatalogIsVideoOnly(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	entry, ok := idx.Provider("vertex")
	if !ok {
		t.Fatal("vertex is missing from the embedded registry")
	}
	if len(entry.Models) != 4 {
		t.Fatalf("the catalog carries %d models, want the four Veo models", len(entry.Models))
	}
	for _, model := range entry.Models {
		if model.Kind != "video" {
			t.Fatalf("model %q kind = %q, want video", model.ID, model.Kind)
		}
		if model.IsChat() {
			t.Fatalf("model %q reports as a chat model", model.ID)
		}
		if model.ID == "gemini-2.5-flash" {
			t.Fatal("a chat model reached the video-only catalog")
		}
	}
}

// TestRegistry_VertexVideoBlock pins the block the media plane dials: the
// provider's own base URL and the service-account credential form, with the
// bearer placement the reference's video proxy always applies.
func TestRegistry_VertexVideoBlock(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	entry, ok := idx.Provider("vertex")
	if !ok {
		t.Fatal("vertex is missing from the embedded registry")
	}
	video, ok := entry.Media.For(MediaVideo)
	if !ok {
		t.Fatal("the video block is missing")
	}
	if video.BaseURL != "https://aiplatform.googleapis.com" {
		t.Fatalf("video base_url = %q", video.BaseURL)
	}
	if video.AuthType != AuthServiceAccount {
		t.Fatalf("video auth_type = %q, want %q", video.AuthType, AuthServiceAccount)
	}
	if video.AuthHeader != "bearer" {
		t.Fatalf("video auth_header = %q, want bearer", video.AuthHeader)
	}
}
