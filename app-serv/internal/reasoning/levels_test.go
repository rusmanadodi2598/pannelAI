// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/levels_test.go
// @for       The level and budget maps, including every threshold boundary.
// @uses      testing.
// @reason    AGENTS.md §2.1 requires the maps proven beside their port: the
//
//	thresholds are the reference's own (thinking.js) and a boundary that
//	drifts by one changes which level a numeric budget is read as, which
//	is a silent behaviour change no caller can see.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "testing"

// TestEffortToBudget pins the level-to-budget map and the unknown-level answer,
// which must stay distinguishable from a zero budget.
func TestEffortToBudget(t *testing.T) {
	cases := []struct {
		effort string
		want   int
		found  bool
	}{
		{"none", 0, true},
		{"minimal", 512, true},
		{"low", 1024, true},
		{"medium", 8192, true},
		{"high", 24576, true},
		{"xhigh", 32768, true},
		{"max", 128000, true},
		{"HIGH", 24576, true},
		{" high ", 24576, true},
		{"", 0, false},
		{"ultra", 0, false},
		{"off", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.effort, func(t *testing.T) {
			got, found := EffortToBudget(tc.effort)
			if got != tc.want || found != tc.found {
				t.Fatalf("EffortToBudget(%q) = (%d, %v), want (%d, %v)", tc.effort, got, found, tc.want, tc.found)
			}
		})
	}
}

// TestEffortToThinkingLevel pins Gemini's four-value enum and its clamps.
func TestEffortToThinkingLevel(t *testing.T) {
	cases := map[string]string{
		"none": "minimal", "off": "minimal", "minimal": "minimal",
		"low": "low", "medium": "medium", "high": "high",
		"xhigh": "high", "max": "high", " HIGH ": "high",
	}
	for effort, want := range cases {
		if got := EffortToThinkingLevel(effort); got != want {
			t.Fatalf("EffortToThinkingLevel(%q) = %q, want %q", effort, got, want)
		}
	}
}

// TestBudgetToLevel pins every threshold: the midpoints between the documented
// budgets, so max stays reachable.
func TestBudgetToLevel(t *testing.T) {
	cases := []struct {
		budget int
		want   string
	}{
		{-1, ""}, {0, ""},
		{1, "minimal"}, {768, "minimal"}, {769, "low"},
		{4096, "low"}, {4097, "medium"},
		{16384, "medium"}, {16385, "high"},
		{28672, "high"}, {28673, "xhigh"},
		{80384, "xhigh"}, {80385, "max"}, {128000, "max"},
	}
	for _, tc := range cases {
		if got := BudgetToLevel(tc.budget); got != tc.want {
			t.Fatalf("BudgetToLevel(%d) = %q, want %q", tc.budget, got, tc.want)
		}
	}
}

// TestBudgetToEffort pins the coarse three-level reverse map.
func TestBudgetToEffort(t *testing.T) {
	cases := []struct {
		budget int
		want   string
	}{
		{-1, ""}, {0, ""},
		{1, "low"}, {2048, "low"}, {2049, "medium"},
		{16384, "medium"}, {16385, "high"}, {128000, "high"},
	}
	for _, tc := range cases {
		if got := BudgetToEffort(tc.budget); got != tc.want {
			t.Fatalf("BudgetToEffort(%d) = %q, want %q", tc.budget, got, tc.want)
		}
	}
}

// TestNormalizeOpenAILevel pins the two clamped levels and the pass-through.
func TestNormalizeOpenAILevel(t *testing.T) {
	cases := []struct {
		level     string
		supported []string
		want      string
	}{
		{"high", []string{"low", "high"}, "high"},
		{"medium", nil, "medium"},
		{"max", []string{"low", "high", "xhigh", "max"}, "max"},
		{"max", []string{"low", "high", "xhigh"}, "xhigh"},
		{"ultra", []string{"low", "high", "xhigh", "max"}, "max"},
		{"ultra", []string{"low", "high", "xhigh", "ultra"}, "ultra"},
		{"ultra", []string{"low", "high"}, "xhigh"},
	}
	for _, tc := range cases {
		if got := NormalizeOpenAILevel(tc.level, tc.supported); got != tc.want {
			t.Fatalf("NormalizeOpenAILevel(%q, %v) = %q, want %q", tc.level, tc.supported, got, tc.want)
		}
	}
}
