// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_routing_failover_test.go
// @for       Tests for the failover budget's normalization, its stored bounds, and the read-back an operator sees.
// @uses      testing, time.
// @reason    A routing decode replaces the whole group, so a document written before these keys existed carries their zero values rather than their defaults, and a walk handed zero attempts would answer NO_PROVIDER_AVAILABLE for a provider that has credentials. The bounds are refused rather than re-read, because an operator who typed 5000 and got 8 was told a different story than the one they wrote.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import (
	"testing"
	"time"
)

func TestFailoverBudget_Normalized(t *testing.T) {
	cases := []struct {
		name         string
		in           FailoverBudget
		wantTotalMS  int
		wantAttempts int
	}{
		{name: "a stored zero normalizes to the documented budget",
			in: FailoverBudget{}, wantTotalMS: DefaultFailoverTotalMS, wantAttempts: DefaultFailoverAttempts},
		{name: "a configured budget is kept",
			in: FailoverBudget{TotalMS: 45_000, MaxAttempts: 12}, wantTotalMS: 45_000, wantAttempts: 12},
		{name: "the smallest legal budget is kept",
			in:          FailoverBudget{TotalMS: MinFailoverTotalMS, MaxAttempts: MinFailoverAttempts},
			wantTotalMS: MinFailoverTotalMS, wantAttempts: MinFailoverAttempts},
		{name: "the largest legal budget is kept",
			in:          FailoverBudget{TotalMS: MaxFailoverTotalMS, MaxAttempts: MaxFailoverAttempts},
			wantTotalMS: MaxFailoverTotalMS, wantAttempts: MaxFailoverAttempts},
		{name: "a total past the ceiling does not walk for a day",
			in:          FailoverBudget{TotalMS: MaxFailoverTotalMS + 1, MaxAttempts: 4},
			wantTotalMS: DefaultFailoverTotalMS, wantAttempts: 4},
		{name: "an attempt count past the ceiling does not walk every account",
			in:          FailoverBudget{TotalMS: 20_000, MaxAttempts: MaxFailoverAttempts + 1},
			wantTotalMS: 20_000, wantAttempts: DefaultFailoverAttempts},
		{name: "a negative budget is the documented one",
			in:          FailoverBudget{TotalMS: -1, MaxAttempts: -9},
			wantTotalMS: DefaultFailoverTotalMS, wantAttempts: DefaultFailoverAttempts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.Normalized()
			if got.TotalMS != tc.wantTotalMS {
				t.Fatalf("total_ms = %d, want %d", got.TotalMS, tc.wantTotalMS)
			}
			if got.MaxAttempts != tc.wantAttempts {
				t.Fatalf("max_attempts = %d, want %d", got.MaxAttempts, tc.wantAttempts)
			}
		})
	}
}

func TestFailoverBudget_TotalAndAttempts(t *testing.T) {
	budget := FailoverBudget{TotalMS: 45_000, MaxAttempts: 3}
	if got := budget.Total(); got != 45*time.Second {
		t.Fatalf("Total() = %v, want 45s", got)
	}
	if got := budget.Attempts(); got != 3 {
		t.Fatalf("Attempts() = %d, want 3", got)
	}
	// The zero value is usable: a caller that reads no settings still gets a bound.
	if got := (FailoverBudget{}).Total(); got != time.Duration(DefaultFailoverTotalMS)*time.Millisecond {
		t.Fatalf("zero Total() = %v, want the documented budget", got)
	}
}

// TestRoutingSettings_NormalizedFillsFailover pins the upgrade path: a stored
// document that predates the keys must still answer the budget the router applies,
// not the zero that would stop the first walk.
func TestRoutingSettings_NormalizedFillsFailover(t *testing.T) {
	legacy := RoutingSettings{
		ComboStrategy:    ComboFallback,
		ComboStickyLimit: 1,
		StickyLimit:      DefaultStickyLimit,
	}
	got := legacy.Normalized()
	if got.Failover.MaxAttempts != DefaultFailoverAttempts {
		t.Fatalf("legacy failover attempts = %d, want the documented %d",
			got.Failover.MaxAttempts, DefaultFailoverAttempts)
	}
	if got.Failover.TotalMS != DefaultFailoverTotalMS {
		t.Fatalf("legacy failover total = %d, want the documented %d",
			got.Failover.TotalMS, DefaultFailoverTotalMS)
	}
}

func TestValidateRouting_FailoverBounds(t *testing.T) {
	cases := []struct {
		name     string
		budget   FailoverBudget
		wantFail bool
	}{
		{name: "unset inherits the default", budget: FailoverBudget{}},
		{name: "the smallest legal pair passes",
			budget: FailoverBudget{TotalMS: MinFailoverTotalMS, MaxAttempts: MinFailoverAttempts}},
		{name: "the largest legal pair passes",
			budget: FailoverBudget{TotalMS: MaxFailoverTotalMS, MaxAttempts: MaxFailoverAttempts}},
		{name: "a total above the ceiling is refused",
			budget: FailoverBudget{TotalMS: MaxFailoverTotalMS + 1}, wantFail: true},
		{name: "a total below the floor is refused",
			budget: FailoverBudget{TotalMS: MinFailoverTotalMS - 1}, wantFail: true},
		{name: "an attempt count above the ceiling is refused",
			budget: FailoverBudget{MaxAttempts: MaxFailoverAttempts + 1}, wantFail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := defaultRoutingSettings()
			settings.Failover = tc.budget
			err := validateRouting(settings)
			if tc.wantFail && err == nil {
				t.Fatal("validateRouting() = nil, want the out-of-range budget refused")
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("validateRouting() error = %v, want the budget accepted", err)
			}
		})
	}
}

// TestApplyRouting_FailoverNamesOneBoundOnly pins that widening the attempt count
// leaves the clock where it was, the same rule each parking window already follows.
func TestApplyRouting_FailoverNamesOneBoundOnly(t *testing.T) {
	stored := defaultRoutingSettings()
	attempts := 20
	patch := RoutingSettingsPatch{Failover: &FailoverBudgetPatch{MaxAttempts: &attempts}}

	applyRouting(&stored, &patch)

	if stored.Failover.MaxAttempts != 20 {
		t.Fatalf("max_attempts = %d, want the 20 the patch named", stored.Failover.MaxAttempts)
	}
	if stored.Failover.TotalMS != DefaultFailoverTotalMS {
		t.Fatalf("total_ms = %d, want the stored clock left alone", stored.Failover.TotalMS)
	}
}
