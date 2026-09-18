// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/combo_reads_test.go
// @for       The Combo aggregate's read accessors, ordering, and load-path tests.
// @uses      testing, time.
// @reason    The strategy-rule table and the accessor tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the accessor cases moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"testing"
	"time"
)

// TestCombo_UpdateAppliesTheSameRules proves a patch cannot reach a shape a
// create refuses.
func TestCombo_UpdateAppliesTheSameRules(t *testing.T) {
	combo, err := NewCombo("cmb_test", "daily", ComboFallback, 0, "",
		[]ComboModel{comboModel(t, "openai/gpt-4o", 1)}, comboNow)
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	later := comboNow.Add(time.Hour)

	err = combo.Update("daily", ComboFusion, 0, "", []ComboModel{comboModel(t, "openai/gpt-4o", 1)}, later)
	if err == nil {
		t.Fatal("Update() accepted fusion without a judge model")
	}
	if combo.Strategy() != ComboFallback {
		t.Fatalf("Update() left the combo in strategy %q after rejecting the change", combo.Strategy())
	}

	if err := combo.Update("daily", ComboRoundRobin, 2, "", []ComboModel{comboModel(t, "openai/gpt-4o", 1)}, later); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if combo.Strategy() != ComboRoundRobin || combo.StickyLimit() != 2 {
		t.Fatalf("Update() = strategy %q sticky %d, want round_robin/2", combo.Strategy(), combo.StickyLimit())
	}
	if !combo.UpdatedAt().Equal(later) {
		t.Fatalf("UpdatedAt() = %v, want %v", combo.UpdatedAt(), later)
	}
	if !combo.CreatedAt().Equal(comboNow) {
		t.Fatalf("Update() moved CreatedAt to %v", combo.CreatedAt())
	}
}

// TestCombo_ModelsReturnsACopy keeps a caller from reordering the aggregate's
// own list (AGENTS.md §2.2: mutation only through the root).
func TestCombo_ModelsReturnsACopy(t *testing.T) {
	combo, err := NewCombo("cmb_test", "daily", ComboFallback, 0, "",
		[]ComboModel{comboModel(t, "c/third", 30), comboModel(t, "a/first", 10)}, comboNow)
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	models := combo.Models()
	models[0] = RehydrateComboModel("mutated", 0)
	if combo.Refs()[0] != "a/first" {
		t.Fatalf("Models() exposed the aggregate's own slice: %v", combo.Refs())
	}
}

// TestCombo_HasRef finds a reference the combo names, which is how the service
// tells an alias that points at a combo from a coincidental string match.
func TestCombo_HasRef(t *testing.T) {
	combo, err := NewCombo("cmb_test", "daily", ComboFallback, 0, "",
		[]ComboModel{comboModel(t, "openai/gpt-4o", 1), comboModel(t, "other-combo", 2)}, comboNow)
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	cases := []struct {
		name string
		ref  string
		want bool
	}{
		{"a listed model", "openai/gpt-4o", true},
		{"a nested combo name", "other-combo", true},
		{"a surrounding space is trimmed", "  other-combo  ", true},
		{"an unlisted model", "anthropic/claude", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := combo.HasRef(tc.ref); got != tc.want {
				t.Fatalf("HasRef(%q) = %t, want %t", tc.ref, got, tc.want)
			}
		})
	}
}

// TestCombo_NextOrder routes the aggregate's own list through the rotation, so
// the service never reimplements the distribution rule.
func TestCombo_NextOrder(t *testing.T) {
	combo, err := NewCombo("cmb_test", "rotating", ComboRoundRobin, 1, "",
		[]ComboModel{comboModel(t, "a/one", 1), comboModel(t, "b/two", 2)}, comboNow)
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	order, next := combo.NextOrder(RotationState{Index: 1})
	if !equalRefs(order, []string{"b/two", "a/one"}) {
		t.Fatalf("NextOrder() = %v, want the rotated order", order)
	}
	if next != (RotationState{Index: 0}) {
		t.Fatalf("NextOrder() state = %+v, want the index wrapped to 0", next)
	}
}

// TestRehydrateCombo_LoadsARowThatPredatesTheRules documents the load path's
// deliberate exemption: a stored row that violates today's rules must still
// load, so the API can report and repair it instead of failing the list.
func TestRehydrateCombo_LoadsARowThatPredatesTheRules(t *testing.T) {
	combo := RehydrateCombo("cmb_old", "legacy", ComboFallback, 0, "",
		[]ComboModel{RehydrateComboModel("openai/gpt-4o", 1)}, comboNow, comboNow)
	if combo.Name() != "legacy" || combo.ModelCount() != 1 {
		t.Fatalf("RehydrateCombo() = %q/%d, want legacy/1", combo.Name(), combo.ModelCount())
	}
	if combo.JudgeModel() != "" {
		t.Fatalf("RehydrateCombo() judge = %q, want empty", combo.JudgeModel())
	}
}

// TestComboModel_RejectsBadShapes covers the entry-level rules on their own.
func TestComboModel_RejectsBadShapes(t *testing.T) {
	cases := []struct {
		name     string
		ref      string
		priority int
		wantErr  bool
	}{
		{"a provider/model ref", "openai/gpt-4o", 1, false},
		{"a combo name", "daily", 1, false},
		{"a dotted alias", "gpt.4o", 0, false},
		{"a negative priority", "openai/gpt-4o", -1, true},
		{"an empty ref", "", 1, true},
		{"a ref with a bad character", "daily!", 1, true},
		{"a ref with a slash but no model", "openai/", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewComboModel(tc.ref, tc.priority)
			if tc.wantErr && err == nil {
				t.Fatalf("NewComboModel(%q, %d) accepted an invalid entry", tc.ref, tc.priority)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewComboModel(%q, %d) error = %v", tc.ref, tc.priority, err)
			}
		})
	}
}

// equalRefs compares two reference lists.
func equalRefs(a, b []string) bool {
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
