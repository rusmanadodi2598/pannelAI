// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/model_catalog_resolve_test.go
// @for       The model-string resolution tests for §7.15.
// @uses      internal/domain, context, strings, testing, time.
// @reason    The set-replacement tests and the resolution tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestModelCatalogService_Resolve covers §7.15's resolution order and the rules
// that a disabled model does not resolve and an unknown string does not either.
func TestModelCatalogService_Resolve(t *testing.T) {
	fixture := newCatalogFixture(t)
	ctx := context.Background()
	if err := fixture.combos.Create(ctx, mustCombo(t, "cmb_one", "fallback-combo")); err != nil {
		t.Fatalf("seeding a combo: %v", err)
	}
	if err := fixture.service.ReplaceAliases(ctx, []domain.ModelAlias{
		mustAlias(t, "fast", "openai/gpt-4o-mini"),
		mustAlias(t, "daily", "fallback-combo"),
	}); err != nil {
		t.Fatalf("ReplaceAliases() error = %v", err)
	}

	cases := []struct {
		name      string
		model     string
		wantKind  RefKind
		wantCombo string
		wantModel string
		wantOK    bool
	}{
		{name: "a combo name", model: "fallback-combo", wantKind: RefKindCombo, wantCombo: "fallback-combo", wantOK: true},
		{name: "an alias to a model", model: "fast", wantKind: RefKindAlias, wantModel: "openai/gpt-4o-mini", wantOK: true},
		{name: "an alias to a combo", model: "daily", wantKind: RefKindAlias, wantCombo: "fallback-combo", wantOK: true},
		{name: "a provider/model string", model: "openai/gpt-4o", wantKind: RefKindModel, wantModel: "openai/gpt-4o", wantOK: true},
		{name: "surrounding whitespace is trimmed", model: "  openai/gpt-4o  ", wantKind: RefKindModel, wantModel: "openai/gpt-4o", wantOK: true},
		{name: "a disabled model does not resolve", model: "anthropic/claude-3", wantOK: false},
		{name: "an unknown model", model: "openai/ghost", wantOK: false},
		{name: "an unknown name", model: "ghost", wantOK: false},
		{name: "an empty string", model: "", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, ok, err := fixture.service.Resolve(ctx, tc.model)
			if tc.model == "" {
				if err == nil {
					t.Fatal("Resolve(\"\") accepted an empty model")
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", tc.model, err)
			}
			if ok != tc.wantOK {
				t.Fatalf("Resolve(%q) ok = %t, want %t", tc.model, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if target.Kind != tc.wantKind {
				t.Fatalf("Resolve(%q) kind = %q, want %q", tc.model, target.Kind, tc.wantKind)
			}
			if tc.wantCombo != "" && target.ComboName != tc.wantCombo {
				t.Fatalf("Resolve(%q) combo = %q, want %q", tc.model, target.ComboName, tc.wantCombo)
			}
			if tc.wantModel != "" && target.Model.String() != tc.wantModel {
				t.Fatalf("Resolve(%q) model = %q, want %q", tc.model, target.Model.String(), tc.wantModel)
			}
		})
	}
}

// TestModelCatalogService_ResolveRejectsAnAliasChain documents the one-level
// dereference rule on the read path: an alias whose target is another alias
// cannot resolve, because such a row is refused at write time.
func TestModelCatalogService_ResolveRejectsAnAliasChain(t *testing.T) {
	fixture := newCatalogFixture(t)
	ctx := context.Background()
	// Written straight to the fake, because the service refuses it: this pins
	// the read path's behaviour for a row that predates the rule.
	if err := fixture.repo.ReplaceAliases(ctx, []domain.ModelAlias{mustAlias(t, "chain", "fast")}); err != nil {
		t.Fatalf("ReplaceAliases() error = %v", err)
	}
	if _, _, err := fixture.service.Resolve(ctx, "chain"); err == nil {
		t.Fatal("Resolve() followed an alias to another alias")
	}
}

// TestModelCatalogService_ModelExists covers the predicate a combo ref, a judge
// model, and a vision adapter entry all validate through.
func TestModelCatalogService_ModelExists(t *testing.T) {
	fixture := newCatalogFixture(t)
	ctx := context.Background()
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "a registry model", raw: "openai/gpt-4o", want: true},
		{name: "a custom model", raw: "openai/local-embed", want: true},
		{name: "a disabled model is not offered", raw: "anthropic/claude-3", want: false},
		{name: "an unknown model", raw: "openai/ghost", want: false},
		{name: "a zero reference", raw: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := domain.ParseModelRef(tc.raw)
			if err != nil {
				ref = domain.ModelRef{}
			}
			got, err := fixture.service.ModelExists(ctx, ref)
			if err != nil {
				t.Fatalf("ModelExists(%q) error = %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("ModelExists(%q) = %t, want %t", tc.raw, got, tc.want)
			}
		})
	}
}

// TestNewModelCatalogService_RequiresDeps pins the constructor's validation.
func TestNewModelCatalogService_RequiresDeps(t *testing.T) {
	index := testIndex(t, testProvider("openai", "api"))
	repo := newStubCatalogRepo()
	combos := newStubComboRepo()
	cases := []struct {
		name string
		deps ModelCatalogServiceDeps
	}{
		{name: "no registry", deps: ModelCatalogServiceDeps{Repo: repo, Combos: combos}},
		{name: "no repository", deps: ModelCatalogServiceDeps{Index: index, Combos: combos}},
		{name: "no combo repository", deps: ModelCatalogServiceDeps{Index: index, Repo: repo}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewModelCatalogService(tc.deps); err == nil {
				t.Fatalf("NewModelCatalogService() accepted %q", tc.name)
			}
		})
	}
	if _, err := NewModelCatalogService(ModelCatalogServiceDeps{Index: index, Repo: repo, Combos: combos}); err != nil {
		t.Fatalf("NewModelCatalogService() error = %v", err)
	}
}

// catalogKeys reduces catalog rows to their provider/model strings, in order.
func catalogKeys(models []domain.CatalogModel) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		out = append(out, model.Ref().String())
	}
	return out
}
