// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_provider_patch_test.go
// @for       The §7.10 per-kind override save and the constructor's guard.
// @uses      testing, context, strings, internal/domain, internal/registry.
// @reason    A save is the one §7.10 management write, and its refusals are
//
//	silent failures if wrong: an unusable provider stored anyway, or a
//	model the kind does not declare. The reads live beside this file.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

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
