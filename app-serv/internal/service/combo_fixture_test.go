// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_fixture_test.go
// @for       The combo fixture: the wired combo service and the reference
//
//	helpers every §7.7 test builds its draft from.
//
// @uses      internal/domain, internal/repository, context, testing.
// @reason    A combo's refs may be a model, another combo, or an alias, so every
//
//	test needs all three present before it can create anything. Naming
//	that setup once keeps each case's table about the rule it pins.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// newComboFixture wires a combo service over in-memory repositories. A nil
// rotation store exercises the documented no-Redis deployment.
func newComboFixture(t *testing.T, rotation repository.ComboRotationStore) (*ComboService, catalogFixture) {
	t.Helper()
	catalog := newCatalogFixture(t)
	service, err := NewComboService(ComboServiceDeps{Repo: catalog.combos, Catalog: catalog.service, Rotation: rotation})
	if err != nil {
		t.Fatalf("NewComboService() error = %v", err)
	}
	return service, catalog
}

// seedComboReferences creates the combo and the aliases a combo's refs may name.
func seedComboReferences(t *testing.T, service *ComboService) {
	t.Helper()
	ctx := context.Background()
	if _, err := service.Create(ctx, comboDraft(t, "seeded-combo", domain.ComboFallback, 0, "", comboRef(t, "openai/gpt-4o", 1))); err != nil {
		t.Fatalf("seeding a combo: %v", err)
	}
	if err := service.catalog.ReplaceAliases(ctx, []domain.ModelAlias{
		mustAlias(t, "fast", "openai/gpt-4o-mini"),
		mustAlias(t, "daily-alias", "seeded-combo"),
	}); err != nil {
		t.Fatalf("seeding the aliases: %v", err)
	}
}

// comboDraft builds a ComboDraft from references.
func comboDraft(t *testing.T, name string, strategy domain.ComboStrategy, sticky int, judge string, models ...domain.ComboModel) ComboDraft {
	t.Helper()
	return ComboDraft{Name: name, Strategy: strategy, StickyLimit: sticky, JudgeModel: judge, Models: models}
}

// comboRef builds one combo model entry or fails the test.
func comboRef(t *testing.T, ref string, priority int) domain.ComboModel {
	t.Helper()
	value, err := domain.NewComboModel(ref, priority)
	if err != nil {
		t.Fatalf("NewComboModel(%q, %d) error = %v", ref, priority, err)
	}
	return value
}

// mustAlias builds one alias mapping.
func mustAlias(t *testing.T, alias, target string) domain.ModelAlias {
	t.Helper()
	value, err := domain.NewModelAlias(alias, target)
	if err != nil {
		t.Fatalf("NewModelAlias(%q, %q) error = %v", alias, target, err)
	}
	return value
}

// mustCombo builds a single-model fallback combo.
func mustCombo(t *testing.T, id, name string) domain.Combo {
	t.Helper()
	model, err := domain.NewComboModel("openai/gpt-4o", 1)
	if err != nil {
		t.Fatalf("NewComboModel() error = %v", err)
	}
	combo, err := domain.NewCombo(id, name, domain.ComboFallback, 0, "", []domain.ComboModel{model}, catalogTestNow())
	if err != nil {
		t.Fatalf("NewCombo(%q) error = %v", name, err)
	}
	return combo
}

// equalStrings compares two string slices in order.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
