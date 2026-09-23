// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_test.go
// @for       Table-driven tests for the merged catalog, its filters, and the (first half; split at the AGENTS.md §1.1 line limit).
// @uses      internal/domain, context, strings, testing, time.
// @reason    The catalog is a merge of three disagreeing sources, and the panel
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestModelCatalogService_MergesThreeSources proves the merged view: registry
// models, custom models, and the disabled subtraction.
func TestModelCatalogService_MergesThreeSources(t *testing.T) {
	fixture := newCatalogFixture(t, context.Background())
	models, err := fixture.service.Catalog(context.Background(), CatalogFilter{})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	got := catalogKeys(models)
	want := []string{
		"black-forest-labs/flux-kontext-pro",
		"openai/gpt-4o",
		"openai/gpt-4o-mini",
		"openai/local-embed",
	}
	if !equalStrings(got, want) {
		t.Fatalf("Catalog() = %v, want %v (anthropic/claude-3 is disabled)", got, want)
	}
	for _, model := range models {
		if model.ProviderID() == "anthropic" {
			t.Fatalf("Catalog() returned the disabled model %q", model.Ref().String())
		}
	}
}

// TestModelCatalogService_CustomOverridesARegistryRow pins the merge precedence:
// the operator's row wins for a pair the registry also declares, because a
// custom row is an explicit statement and a registry row is the port's default.
func TestModelCatalogService_CustomOverridesARegistryRow(t *testing.T) {
	fixture := newCatalogFixture(t, context.Background())
	override := mustCustomModel(t, "openai", "gpt-4o", "Our GPT-4o", "vision")
	if err := fixture.repo.AddCustom(context.Background(), override); err != nil {
		t.Fatalf("AddCustom() error = %v", err)
	}
	models, err := fixture.service.Catalog(context.Background(), CatalogFilter{ProviderID: "openai"})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	var found domain.CatalogModel
	for _, model := range models {
		if model.ModelID() == "gpt-4o" {
			found = model
		}
	}
	if found.DisplayName() != "Our GPT-4o" || found.Source() != domain.CatalogSourceCustom {
		t.Fatalf("merged row = %q/%q, want the custom row to win", found.DisplayName(), found.Source())
	}
}

// TestModelCatalogService_CatalogFilters covers the three documented query
// parameters and their combinations, including a filter that matches nothing.
func TestModelCatalogService_CatalogFilters(t *testing.T) {
	fixture := newCatalogFixture(t, context.Background())
	cases := []struct {
		name   string
		filter CatalogFilter
		want   []string
	}{
		{name: "no filter returns everything enabled", filter: CatalogFilter{}, want: []string{
			"black-forest-labs/flux-kontext-pro", "openai/gpt-4o", "openai/gpt-4o-mini", "openai/local-embed",
		}},
		{name: "by provider", filter: CatalogFilter{ProviderID: "anthropic"}, want: []string{}},
		{name: "by a provider that exists", filter: CatalogFilter{ProviderID: "openai"}, want: []string{
			"openai/gpt-4o", "openai/gpt-4o-mini", "openai/local-embed",
		}},
		// The two modality names are resolved from the model id, so the fixture
		// names ids the reference agrees are vision-capable rather than writing
		// "vision" into its own data (draft 017 §4.4).
		// anthropic/claude-3 is the fixture's disabled row, so it is correctly
		// absent from both answers: the disabled set subtracts before the
		// capability filter runs.
		{name: "by capability", filter: CatalogFilter{Capability: "vision"}, want: []string{
			"openai/gpt-4o", "openai/gpt-4o-mini",
		}},
		{name: "by capability, case-insensitively", filter: CatalogFilter{Capability: "VISION"}, want: []string{
			"openai/gpt-4o", "openai/gpt-4o-mini",
		}},
		// A media operation is a document fact, so it filters from the declared
		// set and not from the model-id resolver.
		{name: "by a media capability the document declares", filter: CatalogFilter{Capability: "edit"}, want: []string{
			"black-forest-labs/flux-kontext-pro",
		}},
		{name: "by an unknown capability", filter: CatalogFilter{Capability: "audio"}, want: []string{}},
		{name: "by free text on the model id", filter: CatalogFilter{Query: "mini"}, want: []string{"openai/gpt-4o-mini"}},
		{name: "by free text on the display name", filter: CatalogFilter{Query: "Local"}, want: []string{"openai/local-embed"}},
		{name: "free text is case-insensitive", filter: CatalogFilter{Query: "EMBED"}, want: []string{"openai/local-embed"}},
		{name: "a filter that matches nothing", filter: CatalogFilter{Query: "nonexistent"}, want: []string{}},
		{
			name:   "provider and capability together",
			filter: CatalogFilter{ProviderID: "openai", Capability: "tools"},
			want:   []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
		},
		{
			name:   "provider and free text together",
			filter: CatalogFilter{ProviderID: "openai", Query: "gpt-4o"},
			want:   []string{"openai/gpt-4o", "openai/gpt-4o-mini"},
		},
		{
			name:   "a provider that does not hold the capability",
			filter: CatalogFilter{ProviderID: "openai", Query: "embed", Capability: "vision"},
			want:   []string{},
		},
		{name: "surrounding whitespace is trimmed", filter: CatalogFilter{ProviderID: "  openai  ", Query: " mini "}, want: []string{"openai/gpt-4o-mini"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models, err := fixture.service.Catalog(context.Background(), tc.filter)
			if err != nil {
				t.Fatalf("Catalog() error = %v", err)
			}
			if got := catalogKeys(models); !equalStrings(got, tc.want) {
				t.Fatalf("Catalog(%+v) = %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}

// TestModelCatalogService_AddCustom pins the create path, including the unknown
// provider rejection §6 requires.
func TestModelCatalogService_AddCustom(t *testing.T) {
	ctx := context.Background()
	fixture := newCatalogFixture(t, ctx)
	cases := []struct {
		name        string
		providerID  string
		modelID     string
		displayName string
		wantErr     string
	}{
		{name: "a new model under a known provider", providerID: "openai", modelID: "gpt-5", displayName: "GPT-5"},
		{name: "a duplicate pair", providerID: "openai", modelID: "gpt-5", displayName: "GPT-5 again", wantErr: "already exists"},
		{name: "an unknown provider", providerID: "nope", modelID: "x", displayName: "X", wantErr: "unknown provider_id"},
		{name: "an empty provider", providerID: "", modelID: "x", displayName: "X", wantErr: "provider_id is required"},
		{name: "an empty model", providerID: "openai", modelID: "", displayName: "X", wantErr: "model_id is required"},
		{name: "an empty display name", providerID: "openai", modelID: "x", displayName: "  ", wantErr: "display_name is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := fixture.service.AddCustom(ctx, tc.providerID, tc.modelID, tc.displayName,
				domain.NewModelCapabilities("vision"))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("AddCustom() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("AddCustom() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AddCustom() error = %v", err)
			}
			if !strings.HasPrefix(model.ID(), domain.CustomModelIDPrefix) {
				t.Fatalf("AddCustom() id = %q, want the %s prefix", model.ID(), domain.CustomModelIDPrefix)
			}
		})
	}
}

// TestModelCatalogService_RemoveCustom covers both outcomes.
func TestModelCatalogService_RemoveCustom(t *testing.T) {
	ctx := context.Background()
	fixture := newCatalogFixture(t, ctx)
	model, err := fixture.service.AddCustom(ctx, "openai", "gone", "Gone", nil)
	if err != nil {
		t.Fatalf("AddCustom() error = %v", err)
	}
	cases := []struct {
		name    string
		id      string
		wantErr string
	}{
		{name: "removing an existing model", id: model.ID()},
		{name: "removing it again", id: model.ID(), wantErr: "model not found"},
		{name: "a blank id", id: "  ", wantErr: "id is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fixture.service.RemoveCustom(ctx, tc.id)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("RemoveCustom() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("RemoveCustom(%q) accepted", tc.id)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("RemoveCustom(%q) error = %v, want %q", tc.id, err, tc.wantErr)
			}
		})
	}
}
