// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/combo_test.go
// @for       Table-driven tests for the Combo aggregate's strategy rules. (first half; split at the AGENTS.md §1.1 line limit).
// @uses      testing, time.
// @reason    SPEC-API-001 §7.7 gives each strategy a different required field
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"testing"
	"time"
)

var comboNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// comboModel is a test shorthand for one ordered entry.
func comboModel(t *testing.T, ref string, priority int) ComboModel {
	t.Helper()
	model, err := NewComboModel(ref, priority)
	if err != nil {
		t.Fatalf("NewComboModel(%q, %d) error = %v", ref, priority, err)
	}
	return model
}

// TestNewCombo_StrategyRules is the rule table of §7.7: which strategies accept
// a judge, which need a sticky limit, and that zero models is refused.
func TestNewCombo_StrategyRules(t *testing.T) {
	models := func(t *testing.T) []ComboModel {
		return []ComboModel{comboModel(t, "openai/gpt-4o", 1), comboModel(t, "anthropic/claude", 2)}
	}
	cases := []struct {
		name        string
		comboName   string
		strategy    ComboStrategy
		stickyLimit int
		judgeModel  string
		models      []ComboModel
		wantErr     string
	}{
		{
			name: "fallback accepts models only", comboName: "daily", strategy: ComboFallback,
			stickyLimit: 0, models: models(t),
		},
		{
			name: "fallback rejects a judge model", comboName: "daily", strategy: ComboFallback,
			judgeModel: "openai/gpt-4o", models: models(t),
			wantErr: "does not accept judge_model",
		},
		{
			name: "round_robin accepts a sticky limit", comboName: "rotating", strategy: ComboRoundRobin,
			stickyLimit: 3, models: models(t),
		},
		{
			name: "round_robin rejects a judge model", comboName: "rotating", strategy: ComboRoundRobin,
			stickyLimit: 1, judgeModel: "openai/gpt-4o", models: models(t),
			wantErr: "does not accept judge_model",
		},
		{
			name: "round_robin rejects a zero sticky limit", comboName: "rotating", strategy: ComboRoundRobin,
			stickyLimit: 0, models: models(t),
			wantErr: "sticky_limit of at least 1",
		},
		{
			name: "round_robin rejects a negative sticky limit", comboName: "rotating", strategy: ComboRoundRobin,
			stickyLimit: -2, models: models(t),
			wantErr: "sticky_limit of at least 1",
		},
		{
			name: "fusion requires a judge model", comboName: "panel", strategy: ComboFusion,
			stickyLimit: 0, models: models(t),
			wantErr: "requires judge_model",
		},
		{
			name: "fusion rejects a sticky limit", comboName: "panel", strategy: ComboFusion,
			stickyLimit: 2, judgeModel: "openai/gpt-4o", models: models(t),
			wantErr: "does not accept sticky_limit",
		},
		{
			name: "fusion accepts a judge model and no sticky limit", comboName: "panel", strategy: ComboFusion,
			stickyLimit: 0, judgeModel: "openai/gpt-4o", models: models(t),
		},
		{
			name: "every strategy refuses an empty model list", comboName: "empty", strategy: ComboFallback,
			models:  []ComboModel{},
			wantErr: "at least one model",
		},
		{
			name: "fusion also refuses an empty model list", comboName: "empty", strategy: ComboFusion,
			judgeModel: "openai/gpt-4o", models: nil,
			wantErr: "at least one model",
		},
		{
			name: "an unknown strategy is rejected", comboName: "weird", strategy: "sequential",
			models: models(t), wantErr: "invalid strategy",
		},
		{
			name: "a duplicated reference is rejected", comboName: "dup", strategy: ComboFallback,
			models:  []ComboModel{comboModel(t, "openai/gpt-4o", 1), comboModel(t, "openai/gpt-4o", 2)},
			wantErr: "listed twice",
		},
		{
			name: "a duplicated judge reference is accepted as a model", comboName: "judged", strategy: ComboFusion,
			judgeModel: "openai/gpt-4o",
			models:     []ComboModel{comboModel(t, "openai/gpt-4o", 1)},
		},
		{
			name: "an empty name is rejected", comboName: "   ", strategy: ComboFallback,
			models: models(t), wantErr: "name is required",
		},
		{
			name: "a name with a slash is rejected", comboName: "provider/model", strategy: ComboFallback,
			models: models(t), wantErr: "may only contain",
		},
		{
			name: "a name with whitespace is rejected", comboName: "two words", strategy: ComboFallback,
			models: models(t), wantErr: "may only contain",
		},
		{
			name: "a ref with whitespace is rejected", comboName: "refs", strategy: ComboFallback,
			models:  []ComboModel{{ref: "openai/gpt 4o", priority: 1}},
			wantErr: "must not contain whitespace",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			combo, err := NewCombo("cmb_test", tc.comboName, tc.strategy, tc.stickyLimit, tc.judgeModel, tc.models, comboNow)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("NewCombo() error = %v, want success", err)
				}
				if got := combo.StickyLimit(); tc.strategy != ComboRoundRobin && got != 1 {
					t.Fatalf("StickyLimit() = %d, want the default 1 for %s", got, tc.strategy)
				}
				if combo.Name() != strings.TrimSpace(tc.comboName) {
					t.Fatalf("Name() = %q, want the trimmed input", combo.Name())
				}
				return
			}
			if err == nil {
				t.Fatalf("NewCombo() accepted an invalid combo, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewCombo() error = %v, want it to contain %q", err, tc.wantErr)
			}
			if AsAppError(err).Code != "VALIDATION_ERROR" {
				t.Fatalf("NewCombo() code = %q, want VALIDATION_ERROR", AsAppError(err).Code)
			}
		})
	}
}

// TestNewCombo_OrdersModelsByPriority pins the ordering the router walks: the
// stored slice is sorted, so a fallback tries the operator's first choice first
// whatever order the panel sent.
func TestNewCombo_OrdersModelsByPriority(t *testing.T) {
	combo, err := NewCombo("cmb_test", "ordered", ComboFallback, 0, "",
		[]ComboModel{
			comboModel(t, "c/third", 30),
			comboModel(t, "a/first", 10),
			comboModel(t, "b/second", 20),
		}, comboNow)
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	want := []string{"a/first", "b/second", "c/third"}
	if got := combo.Refs(); !equalRefs(got, want) {
		t.Fatalf("Refs() = %v, want %v", got, want)
	}
	if combo.ModelCount() != 3 {
		t.Fatalf("ModelCount() = %d, want 3", combo.ModelCount())
	}
}

// TestNewCombo_KeepsEqualPrioritiesStable keeps the panel's manual ordering
// through a round-trip: a stable sort leaves equal priorities as they arrived.
func TestNewCombo_KeepsEqualPrioritiesStable(t *testing.T) {
	combo, err := NewCombo("cmb_test", "stable", ComboFallback, 0, "",
		[]ComboModel{
			comboModel(t, "b/second", 5),
			comboModel(t, "a/first", 5),
			comboModel(t, "c/third", 5),
		}, comboNow)
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	want := []string{"b/second", "a/first", "c/third"}
	if got := combo.Refs(); !equalRefs(got, want) {
		t.Fatalf("Refs() = %v, want the arrival order %v", got, want)
	}
}
