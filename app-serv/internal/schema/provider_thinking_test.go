// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider_thinking_test.go
// @for       The §7.14 level sets the panel reads: the union a provider detail
//
//	carries, and the levels a custom-model or catalog row carries.
//
// @uses      testing, internal/domain, internal/registry.
// @reason    The picker's options and the suffix a copied model name gains are
//
//	one decision each, and both are made from a level set the panel cannot
//	compute: the tables live in the registry package. Asserting the two
//	projections here is what keeps a picker from offering a level the
//	upstream would refuse, and a suffix from being appended to a model
//	that does not accept it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-26
package schema

import (
	"reflect"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// thinkingTestTime is a fixed instant, so the rows built here carry a stable
// created_at the assertions do not have to read.
func thinkingTestTime() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) }

// declaredModel builds one row the operator declared, which is the union's
// second source.
func declaredModel(t *testing.T, id, providerID, modelID string) domain.CustomModel {
	t.Helper()
	model, err := domain.NewCustomModel(id, providerID, modelID, modelID, nil, thinkingTestTime())
	if err != nil {
		t.Fatalf("NewCustomModel(%q, %q, %q) error = %v", id, providerID, modelID, err)
	}
	return model
}

// TestProviderDetailFrom_CarriesTheThinkingLevelUnion pins the union rule: the
// levels of every declared model, in discovery order, without duplicates and
// without "none" — which is the absence of a level rather than a choice.
func TestProviderDetailFrom_CarriesTheThinkingLevelUnion(t *testing.T) {
	entry := registry.Provider{
		ID: "anthropic", Category: "apikey", Priority: 1,
		Transport: registry.Transport{Format: "claude"},
		Models: []registry.Model{
			{ID: "claude-opus-4-8", Name: "Opus", Kind: "llm"},
			// A second reasoning model adds the one level the first does not
			// carry, and repeats the rest, which the union must not duplicate.
			{ID: "claude-opus-4-8-preview", Name: "Opus preview", Kind: "llm"},
			// A non-reasoning model contributes nothing at all.
			{ID: "gpt-image-1", Name: "Image", Kind: "llm"},
		},
	}
	got := ProviderDetailFrom(entry, ProviderStatusSummaryDTO{}, nil).ThinkingLevels
	want := []string{"low", "medium", "high", "max", "xhigh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProviderDetailFrom(...).ThinkingLevels = %v, want %v", got, want)
	}

	silent := registry.Provider{ID: "openai", Category: "apikey", Priority: 1,
		Transport: registry.Transport{Format: "openai"},
		Models:    []registry.Model{{ID: "gpt-image-1", Name: "Image", Kind: "llm"}}}
	if got := ProviderDetailFrom(silent, ProviderStatusSummaryDTO{}, nil).ThinkingLevels; got != nil {
		t.Fatalf("a provider whose models do not reason answered %v, want no levels", got)
	}
}

// TestProviderDetailFrom_UnionIncludesTheDeclaredRows pins the union's second
// source. A synthesized custom node carries no registry models at all, so
// without its declared rows its detail answers no level set and the panel hides
// the reasoning picker on exactly the screen whose rows copy the suffix — the
// gap the owner reported on 2026-09-27. A registry provider's declared rows
// join its registry models in the same union, because the panel copies the
// suffix onto those rows too.
func TestProviderDetailFrom_UnionIncludesTheDeclaredRows(t *testing.T) {
	node := registry.Provider{ID: "openai-compatible-01TEST", Category: "apikey", Custom: true,
		Transport: registry.Transport{Format: "openai"}}
	declared := []domain.CustomModel{
		declaredModel(t, "cm_1", node.ID, "claude-opus-4-8"),
		// An id the registry does not know contributes nothing, and the union
		// must not report an empty string for it.
		declaredModel(t, "cm_2", node.ID, "mystery-1"),
	}
	want := []string{"low", "medium", "high", "max"}
	if got := ProviderDetailFrom(node, ProviderStatusSummaryDTO{}, declared).ThinkingLevels; !reflect.DeepEqual(got, want) {
		t.Fatalf("a node's declared rows answered %v, want %v", got, want)
	}

	// A node whose rows are all unknown answers none, which is what hides the picker.
	if got := ProviderDetailFrom(node, ProviderStatusSummaryDTO{},
		[]domain.CustomModel{declared[1]}).ThinkingLevels; got != nil {
		t.Fatalf("a node with only unknown rows answered %v, want no levels", got)
	}

	// A registry provider whose own models do not reason still offers the levels
	// its declared rows accept.
	silent := registry.Provider{ID: "openai", Category: "apikey", Priority: 1,
		Transport: registry.Transport{Format: "openai"},
		Models:    []registry.Model{{ID: "gpt-image-1", Name: "Image", Kind: "llm"}}}
	if got := ProviderDetailFrom(silent, ProviderStatusSummaryDTO{}, declared[:1]).ThinkingLevels; !reflect.DeepEqual(got, want) {
		t.Fatalf("a registry provider's declared rows answered %v, want %v", got, want)
	}
}

// TestToCustomModelResponse_CarriesTheModelsOwnLevels pins the per-row answer:
// a custom model that names a known model carries that model's levels, and one
// the registry does not know carries none — the honest answer, because the
// panel would otherwise append a suffix the upstream refuses.
func TestToCustomModelResponse_CarriesTheModelsOwnLevels(t *testing.T) {
	known, err := domain.NewCustomModel("cm_1", "anthropic", "claude-opus-4-8", "Opus", nil, thinkingTestTime())
	if err != nil {
		t.Fatalf("NewCustomModel() error = %v", err)
	}
	got := ToCustomModelResponse(known)
	want := []string{"low", "medium", "high", "max"}
	if !reflect.DeepEqual(got.ThinkingLevels, want) {
		t.Fatalf("ThinkingLevels = %v, want %v", got.ThinkingLevels, want)
	}

	unknown, err := domain.NewCustomModel("cm_2", "openai-compatible-01TEST", "mystery-1", "Mystery", nil, thinkingTestTime())
	if err != nil {
		t.Fatalf("NewCustomModel() error = %v", err)
	}
	if got := ToCustomModelResponse(unknown).ThinkingLevels; got != nil {
		t.Fatalf("an unknown model answered %v, want no levels", got)
	}
}

// TestToModelResponses_CarriesEachRowsOwnLevels pins the catalog projection:
// each row answers its own model's levels, so a copied name can carry a suffix
// only where the model accepts it, and a row the registry does not know carries
// none rather than the provider's union.
func TestToModelResponses_CarriesEachRowsOwnLevels(t *testing.T) {
	known, err := domain.NewModelRef("anthropic", "claude-opus-4-8")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	unknown, err := domain.NewModelRef("openai-compatible-01TEST", "mystery-1")
	if err != nil {
		t.Fatalf("NewModelRef() error = %v", err)
	}
	rows := ToModelResponses([]domain.CatalogModel{
		domain.NewCatalogModel(known, "Opus", "llm", nil, domain.CatalogSourceRegistry),
		domain.NewCatalogModel(unknown, "Mystery", "llm", nil, domain.CatalogSourceRegistry),
	})
	want := []string{"low", "medium", "high", "max"}
	if !reflect.DeepEqual(rows[0].ThinkingLevels, want) {
		t.Fatalf("rows[0].ThinkingLevels = %v, want %v", rows[0].ThinkingLevels, want)
	}
	if rows[1].ThinkingLevels != nil {
		t.Fatalf("an unknown model answered %v, want no levels", rows[1].ThinkingLevels)
	}
}
