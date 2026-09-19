// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_provider_test.go
// @for       The §7.10 media provider reads.
// @uses      testing, context, time, internal/domain.
// @reason    The rules worth pinning are the resolutions: an override wins over
//
//	the registry, an all-empty override reads as "not overridden", and
//	the listing order is canonical. Each is a silent failure if wrong.
//	The save's refusals live in media_provider_patch_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// mediaNow is the fixed clock the override cases store.
func mediaNow() time.Time { return time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC) }

// TestMediaProviderService_List pins the list: one view per provider and kind,
// in the canonical kind order, with hidden and media-less providers skipped.
func TestMediaProviderService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("every kind", func(t *testing.T) {
		svc, _ := mediaFixture(t, mediaProviderEntries(), nil)
		views, err := svc.List(ctx, "")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		want := []struct {
			provider string
			kind     domain.MediaKind
		}{
			{"openai", domain.MediaKindTTS},
			{"openai", domain.MediaKindEmbedding},
			{"openai", domain.MediaKindImage},
			{"elevenlabs", domain.MediaKindTTS},
		}
		if len(views) != len(want) {
			t.Fatalf("List() returned %d views, want %d: %+v", len(views), len(want), views)
		}
		for index, view := range views {
			if view.ProviderID != want[index].provider || view.Kind != want[index].kind {
				t.Fatalf("List()[%d] = %s/%s, want %s/%s", index,
					view.ProviderID, view.Kind, want[index].provider, want[index].kind)
			}
		}
		if views[0].BaseURL != "https://api.openai.com/v1" || views[0].DefaultModel != "tts-1" {
			t.Fatalf("List()[0] = %+v, want the registry's base URL and default model", views[0])
		}
		if views[0].BaseURLSource != domain.MediaSourceRegistry || views[0].DefaultModelSource != domain.MediaSourceRegistry {
			t.Fatalf("List()[0] sources = %s/%s, want registry/registry with no stored row",
				views[0].BaseURLSource, views[0].DefaultModelSource)
		}
	})

	t.Run("one kind", func(t *testing.T) {
		svc, _ := mediaFixture(t, mediaProviderEntries(), nil)
		views, err := svc.List(ctx, domain.MediaKindImage)
		if err != nil {
			t.Fatalf("List(image) error = %v", err)
		}
		if len(views) != 1 || views[0].ProviderID != "openai" || views[0].Kind != domain.MediaKindImage {
			t.Fatalf("List(image) = %+v, want only openai/image", views)
		}
	})

	t.Run("a kind nobody offers", func(t *testing.T) {
		svc, _ := mediaFixture(t, mediaProviderEntries(), nil)
		views, err := svc.List(ctx, domain.MediaKindSearch)
		if err != nil {
			t.Fatalf("List(search) error = %v", err)
		}
		if len(views) != 0 {
			t.Fatalf("List(search) = %+v, want an empty set", views)
		}
	})
}

// TestMediaProviderService_ListResolvesOverrides pins that the stored override
// wins, and that an all-empty row reads as "not overridden" rather than as a
// blank base URL.
func TestMediaProviderService_ListResolvesOverrides(t *testing.T) {
	ctx := context.Background()
	svc, repo := mediaFixture(t, mediaProviderEntries(), nil)

	stored, err := domain.NewMediaOverride("openai", domain.MediaKindTTS,
		"https://tts.example.com/v1", "tts-2", mediaNow())
	if err != nil {
		t.Fatalf("NewMediaOverride() error = %v", err)
	}
	if err := repo.Upsert(ctx, stored); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	blank, err := domain.NewMediaOverride("elevenlabs", domain.MediaKindTTS, "", "", mediaNow())
	if err != nil {
		t.Fatalf("NewMediaOverride() error = %v", err)
	}
	if err := repo.Upsert(ctx, blank); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	views, err := svc.List(ctx, domain.MediaKindTTS)
	if err != nil {
		t.Fatalf("List(tts) error = %v", err)
	}
	if views[0].BaseURL != "https://tts.example.com/v1" || views[0].DefaultModel != "tts-2" {
		t.Fatalf("openai view = %+v, want the override resolved", views[0])
	}
	if views[0].BaseURLSource != domain.MediaSourceOverride || views[0].DefaultModelSource != domain.MediaSourceOverride {
		t.Fatalf("openai sources = %s/%s, want override/override", views[0].BaseURLSource, views[0].DefaultModelSource)
	}
	if views[1].BaseURL != "https://api.elevenlabs.io/v1" {
		t.Fatalf("elevenlabs view = %+v, want the registry default", views[1])
	}
	if views[1].BaseURLSource != domain.MediaSourceRegistry || views[1].DefaultModelSource != domain.MediaSourceRegistry {
		t.Fatalf("elevenlabs sources = %s/%s, want registry/registry for an all-empty row",
			views[1].BaseURLSource, views[1].DefaultModelSource)
	}
}

// TestMediaProviderService_ListCountsEndpoints pins the roll-up read: one query
// keyed by the page's providers, reported per view.
func TestMediaProviderService_ListCountsEndpoints(t *testing.T) {
	counts := stubMediaCounts{byProvider: map[string]domain.EndpointStatusCounts{
		"openai":     {Total: 2, Active: 2},
		"elevenlabs": {Total: 1, Active: 1},
	}}
	svc, _ := mediaFixture(t, mediaProviderEntries(), counts)
	views, err := svc.List(context.Background(), domain.MediaKindTTS)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if views[0].EndpointCount != 2 || views[1].EndpointCount != 1 {
		t.Fatalf("endpoint counts = %d/%d, want 2/1", views[0].EndpointCount, views[1].EndpointCount)
	}
}

// TestMediaProviderService_Detail pins the detail reads and both not-found
// paths: an unknown provider, and a provider that offers no media service.
func TestMediaProviderService_Detail(t *testing.T) {
	ctx := context.Background()
	svc, _ := mediaFixture(t, mediaProviderEntries(), nil)

	views, err := svc.Detail(ctx, "openai")
	if err != nil {
		t.Fatalf("Detail(openai) error = %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("Detail(openai) returned %d views, want 3", len(views))
	}
	for _, view := range views {
		if view.ProviderID != "openai" || view.ProviderName != "OpenAI" {
			t.Fatalf("Detail(openai) view = %+v, want the provider identity", view)
		}
	}

	if _, err := svc.Detail(ctx, "ghost"); domain.AsAppError(err).Code != "NOT_FOUND" {
		t.Fatalf("Detail(ghost) error = %v, want not found", err)
	}
	if _, err := svc.Detail(ctx, "chatonly"); err == nil {
		t.Fatal("Detail(chatonly) accepted a provider with no media service")
	}
}

// The save rules — Patch, the unusable-provider refusal, and the constructor's
// guard — live in media_provider_patch_test.go, beside the writes they pin.
