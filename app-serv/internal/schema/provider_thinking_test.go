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
	got := ProviderDetailFrom(entry, ProviderStatusSummaryDTO{}).ThinkingLevels
	want := []string{"low", "medium", "high", "max", "xhigh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProviderDetailFrom(...).ThinkingLevels = %v, want %v", got, want)
	}

	silent := registry.Provider{ID: "openai", Category: "apikey", Priority: 1,
		Transport: registry.Transport{Format: "openai"},
		Models:    []registry.Model{{ID: "gpt-image-1", Name: "Image", Kind: "llm"}}}
	if got := ProviderDetailFrom(silent, ProviderStatusSummaryDTO{}).ThinkingLevels; got != nil {
		t.Fatalf("a provider whose models do not reason answered %v, want no levels", got)
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
