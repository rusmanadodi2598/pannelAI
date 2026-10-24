// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/provider_agnes_test.go
// @for       The Agnes AI entry, its catalog split, and the media blocks it is served with.
// @uses      testing, internal/registry.
// @reason    Agnes AI is an app-serv declaration the reference carries no entry for, so its transport, model kinds, and credential placement have no second copy to compare against; pinning them here catches a hand edit that breaks the entry between generator runs.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-21
package registry

import "testing"

// TestRegistry_AgnesEntry pins the transport, the model catalog split, and the
// media blocks the provider is served with.
func TestRegistry_AgnesEntry(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	entry, ok := idx.Provider("agnes")
	if !ok {
		t.Fatal("agnes is missing from the embedded registry")
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
	if entry.Transport.BaseURL != "https://apihub.agnes-ai.com/v1/chat/completions" {
		t.Fatalf("base_url = %q", entry.Transport.BaseURL)
	}
	if entry.Transport.ValidateURL != "https://apihub.agnes-ai.com/v1/models" {
		t.Fatalf("validate_url = %q, want the models endpoint the provider validates a key against", entry.Transport.ValidateURL)
	}

	for _, id := range []string{"agnes-2.5-flash", "agnes-3.0-flash"} {
		model, ok := idx.Model("agnes", id)
		if !ok {
			t.Fatalf("chat model %q is missing", id)
		}
		if !model.IsChat() {
			t.Fatalf("model %q must be routable through the chat data plane", id)
		}
		if model.ContextLength != 512000 || model.MaxOutputTokens != 65536 {
			t.Fatalf("model %q declares %d/%d, want 512000/65536", id, model.ContextLength, model.MaxOutputTokens)
		}
	}
	for _, id := range []string{"agnes-image-2.0-flash", "agnes-image-2.1-flash", "agnes-image-2.5-flash"} {
		model, ok := idx.Model("agnes", id)
		if !ok {
			t.Fatalf("image model %q is missing", id)
		}
		if model.Kind != string(MediaImage) {
			t.Fatalf("model %q kind = %q, want image", id, model.Kind)
		}
	}
	videoModel, ok := idx.Model("agnes", "agnes-video-2.5-flash")
	if !ok {
		t.Fatal("video model agnes-video-2.5-flash is missing")
	}
	if videoModel.Kind != string(MediaVideo) || videoModel.IsChat() {
		t.Fatalf("model agnes-video-2.5-flash must be video-only, got kind %q", videoModel.Kind)
	}

	for _, kind := range []MediaKind{MediaImage, MediaVideo} {
		if !entry.Media.Supports(kind) {
			t.Fatalf("media block %q is missing", kind)
		}
	}
	image, _ := entry.Media.For(MediaImage)
	if image.BaseURL != "https://apihub.agnes-ai.com/v1/images/generations" {
		t.Fatalf("image base_url = %q", image.BaseURL)
	}
	video, _ := entry.Media.For(MediaVideo)
	if video.BaseURL != "https://apihub.agnes-ai.com/v1/videos" {
		t.Fatalf("video base_url = %q, want the create endpoint the poll reuses for its host", video.BaseURL)
	}
	if image.AuthType != "apikey" || image.AuthHeader != "bearer" {
		t.Fatalf("image auth = %q/%q, want apikey/bearer", image.AuthType, image.AuthHeader)
	}
	if video.AuthType != "apikey" || video.AuthHeader != "bearer" {
		t.Fatalf("video auth = %q/%q, want apikey/bearer", video.AuthType, video.AuthHeader)
	}
}
