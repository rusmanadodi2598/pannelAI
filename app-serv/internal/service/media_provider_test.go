// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_provider_test.go
// @for       The §7.10 media provider reads and the per-kind override save.
// @uses      testing, context, errors, internal/domain, internal/registry.
// @reason    The rules worth pinning are the resolutions: an override wins over
//
//	the registry, an all-empty override reads as "not overridden", a
//	save that would leave no base URL is refused, and a model the kind
//	does not declare is refused. Each is a silent failure if wrong.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
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

// TestMediaProviderService_Patch pins the save rules with a parameterized table.
func TestMediaProviderService_Patch(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name            string
		provider        string
		draft           MediaOverrideDraft
		wantErr         string
		wantURL         string
		wantURLSource   string
		wantModel       string
		wantModelSource string
	}{
		{
			name: "a self-hosted base URL and a declared model", provider: "openai",
			draft:   MediaOverrideDraft{Kind: domain.MediaKindTTS, BaseURL: "http://127.0.0.1:8000/v1", DefaultModel: "tts-2"},
			wantURL: "http://127.0.0.1:8000/v1", wantURLSource: domain.MediaSourceOverride,
			wantModel: "tts-2", wantModelSource: domain.MediaSourceOverride,
		},
		{
			name: "an empty base URL returns to the registry default", provider: "openai",
			draft:   MediaOverrideDraft{Kind: domain.MediaKindTTS, DefaultModel: "tts-1"},
			wantURL: "https://api.openai.com/v1", wantURLSource: domain.MediaSourceRegistry,
			wantModel: "tts-1", wantModelSource: domain.MediaSourceOverride,
		},
		{
			name: "an unknown provider", provider: "ghost",
			draft:   MediaOverrideDraft{Kind: domain.MediaKindTTS, BaseURL: "https://tts.example.com"},
			wantErr: "not in the registry",
		},
		{
			name: "a kind the provider does not offer", provider: "elevenlabs",
			draft:   MediaOverrideDraft{Kind: domain.MediaKindImage, BaseURL: "https://img.example.com"},
			wantErr: "does not offer image",
		},
		{
			name: "a model the kind does not declare", provider: "openai",
			draft:   MediaOverrideDraft{Kind: domain.MediaKindTTS, DefaultModel: "ghost-model"},
			wantErr: "not declared",
		},
		{
			name: "a base URL of the wrong shape", provider: "openai",
			draft:   MediaOverrideDraft{Kind: domain.MediaKindTTS, BaseURL: "ftp://tts.example.com"},
			wantErr: "http or https",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := mediaFixture(t, mediaProviderEntries(), nil)
			view, err := svc.Patch(ctx, tc.provider, tc.draft)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Patch(%s) accepted, want a refusal naming %q", tc.name, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to name %q", err.Error(), tc.wantErr)
				}
				if repo.upserts != 0 {
					t.Fatalf("a refused save wrote %d rows", repo.upserts)
				}
				return
			}
			if err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
			if view.BaseURL != tc.wantURL || view.BaseURLSource != tc.wantURLSource {
				t.Fatalf("base URL = %s (%s), want %s (%s)", view.BaseURL, view.BaseURLSource, tc.wantURL, tc.wantURLSource)
			}
			if view.DefaultModel != tc.wantModel || view.DefaultModelSource != tc.wantModelSource {
				t.Fatalf("default model = %s (%s), want %s (%s)",
					view.DefaultModel, view.DefaultModelSource, tc.wantModel, tc.wantModelSource)
			}
			if repo.upserts != 1 {
				t.Fatalf("upserts = %d, want 1", repo.upserts)
			}
		})
	}
}

// TestMediaProviderService_PatchRefusesAnUnusableProvider pins §7.10's rule
// that a provider with no base URL from either source is refused rather than
// silently falling back.
func TestMediaProviderService_PatchRefusesAnUnusableProvider(t *testing.T) {
	entries := []registry.Provider{{
		ID: "selfhosted", Display: registry.Display{Name: "Self-hosted"}, Media: registry.MediaConfigs{
			registry.MediaTTS: {DefaultModel: "voice-model"},
		},
	}}
	svc, repo := mediaFixture(t, entries, nil)
	_, err := svc.Patch(context.Background(), "selfhosted",
		MediaOverrideDraft{Kind: domain.MediaKindTTS, BaseURL: "", DefaultModel: "voice-model"})
	if err == nil {
		t.Fatal("Patch() accepted a provider with no base URL at all")
	}
	if repo.upserts != 0 {
		t.Fatalf("a refused save wrote %d rows", repo.upserts)
	}
}

// TestNewMediaProviderService_RequiresDeps pins the constructor's guard.
func TestNewMediaProviderService_RequiresDeps(t *testing.T) {
	index := &mediaIndex{entries: mediaProviderEntries()}
	cases := []struct {
		name string
		deps MediaProviderServiceDeps
	}{
		{name: "no index", deps: MediaProviderServiceDeps{Repo: newStubMediaRepo()}},
		{name: "no repository", deps: MediaProviderServiceDeps{Index: index}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewMediaProviderService(tc.deps); err == nil {
				t.Fatalf("NewMediaProviderService(%s) = nil error, want a validation failure", tc.name)
			}
		})
	}
}
