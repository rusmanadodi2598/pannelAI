// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_override_test.go
// @for       The §7.10 override resolution the embeddings use case reads.
// @uses      internal/domain, internal/registry, context, errors, strings,
//
//	testing.
//
// @reason    §7.10's save is only real if the data plane honours it, and the
//
//	rule has three outcomes worth pinning: a stored override wins, an
//	absent one falls back to the registry, and a provider with neither
//	is refused rather than defaulted to a cloud host.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubOverrideReader answers with one fixed value per provider and kind, or
// fails when failure is set.
type stubOverrideReader struct {
	values  map[string]string
	failure error
}

func (r stubOverrideReader) MediaBaseURL(_ context.Context, providerID string, kind domain.MediaKind) (string, error) {
	if r.failure != nil {
		return "", r.failure
	}
	return r.values[providerID+"\x00"+string(kind)], nil
}

// embeddingEntry is the registry entry the resolution cases read. The
// embedding block is always present — an entry that lacks it is the separate
// not-routable case — and its base URL is whatever the case declares.
func embeddingEntry(baseURL string) registry.Provider {
	return registry.Provider{
		ID: "openai", Display: registry.Display{Name: "OpenAI"},
		Media: registry.MediaConfigs{registry.MediaEmbedding: {BaseURL: baseURL}},
	}
}

// TestEmbeddingsService_MediaBaseURLResolution pins where the effective base URL
// comes from. The table is parameterized over the stored value and the registry
// value, with the refusal as its own row.
func TestEmbeddingsService_MediaBaseURLResolution(t *testing.T) {
	cases := []struct {
		name      string
		registry  string
		stored    string
		want      string
		wantError string
	}{
		{name: "registry only", registry: "https://api.openai.com/v1", want: "https://api.openai.com/v1"},
		{
			name: "stored override wins", registry: "https://api.openai.com/v1",
			stored: "http://127.0.0.1:8000/v1", want: "http://127.0.0.1:8000/v1",
		},
		{
			name: "stored override with no registry value", stored: "http://127.0.0.1:8000/v1",
			want: "http://127.0.0.1:8000/v1",
		},
		{
			name: "neither source", wantError: "has no embeddings base_url configured",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &EmbeddingsService{overrides: stubOverrideReader{values: map[string]string{
				"openai\x00embedding": tc.stored,
			}}}
			_, baseURL, err := svc.mediaConfig(context.Background(), embeddingEntry(tc.registry))
			if tc.wantError != "" {
				if err == nil || !containsText(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("mediaConfig() error = %v", err)
			}
			if baseURL != tc.want {
				t.Fatalf("baseURL = %q, want %q", baseURL, tc.want)
			}
		})
	}
}

// TestEmbeddingsService_MediaBaseURLRefusals covers the two answers that are not
// about the URL: a provider with no embeddings service, and a read failure.
func TestEmbeddingsService_MediaBaseURLRefusals(t *testing.T) {
	t.Run("a provider without embeddings", func(t *testing.T) {
		svc := &EmbeddingsService{}
		_, _, err := svc.mediaConfig(context.Background(), registry.Provider{ID: "elevenlabs"})
		if err == nil || !containsText(err.Error(), "does not offer embeddings") {
			t.Fatalf("error = %v, want the not-routable refusal", err)
		}
	})

	t.Run("a read failure surfaces", func(t *testing.T) {
		svc := &EmbeddingsService{overrides: stubOverrideReader{failure: errors.New("store is down")}}
		_, _, err := svc.mediaConfig(context.Background(), embeddingEntry("https://api.openai.com/v1"))
		if err == nil || !containsText(err.Error(), "store is down") {
			t.Fatalf("error = %v, want the read failure", err)
		}
	})
}

// TestMediaProviderService_MediaBaseURL pins the reader the data plane uses: it
// answers only the exact provider and kind, and trims what it finds.
func TestMediaProviderService_MediaBaseURL(t *testing.T) {
	svc, repo := mediaFixture(t, mediaProviderEntries(), nil)
	ctx := context.Background()

	stored, err := svc.MediaBaseURL(ctx, "openai", domain.MediaKindEmbedding)
	if err != nil {
		t.Fatalf("MediaBaseURL() error = %v", err)
	}
	if stored != "" {
		t.Fatalf("stored = %q, want empty before a save", stored)
	}

	if _, err := svc.Patch(ctx, "openai", MediaOverrideDraft{
		Kind: domain.MediaKindEmbedding, BaseURL: "  http://127.0.0.1:9000/v1  ",
	}); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	stored, err = svc.MediaBaseURL(ctx, "openai", domain.MediaKindEmbedding)
	if err != nil {
		t.Fatalf("MediaBaseURL() error = %v", err)
	}
	if stored != "http://127.0.0.1:9000/v1" {
		t.Fatalf("stored = %q, want the saved override", stored)
	}

	// A different kind of the same provider is not the row that was written.
	other, err := svc.MediaBaseURL(ctx, "openai", domain.MediaKindTTS)
	if err != nil {
		t.Fatalf("MediaBaseURL() error = %v", err)
	}
	if other != "" {
		t.Fatalf("stored tts = %q, want empty", other)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("stored rows = %d, want 1", len(repo.rows))
	}
}

// containsText is the substring check the cases above read.
func containsText(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
