// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/usage_types_test.go
// @for       Table-driven tests for aggregation arithmetic and the group-by set.
// @uses      testing, time.
// @reason    AGENTS.md §2.1 and §2.4 require the aggregation the summary, the
//
//	group breakdown, and the timeseries all display to be pinned,
//	including the zero-record range. A summary that is wrong only for
//	an empty window or only for the error rate is exactly the defect a
//	happy-path test misses.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"testing"
	"time"
)

// TestUsageGroupBy_DimensionSet pins the closed dimension set and the column
// each one maps to, which is the only string that reaches SQL.
func TestUsageGroupBy_DimensionSet(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantValid  bool
		wantColumn string
	}{
		{"provider", "provider", true, "provider_id"},
		{"model", "model", true, "model"},
		{"endpoint", "endpoint", true, "endpoint_id"},
		{"gateway key", "gateway_key", true, "gateway_key_id"},
		{"empty is not a dimension", "", false, ""},
		{"unknown", "status", false, ""},
		{"case sensitive", "Provider", false, ""},
		{"sql injection attempt", "provider_id; DROP TABLE usage_records", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			group := UsageGroupBy(tc.in)
			if got := group.IsValid(); got != tc.wantValid {
				t.Fatalf("IsValid(%q) = %v, want %v", tc.in, got, tc.wantValid)
			}
			if got := group.Column(); got != tc.wantColumn {
				t.Fatalf("Column(%q) = %q, want %q", tc.in, got, tc.wantColumn)
			}
			_, err := ParseUsageGroupBy(tc.in)
			if tc.wantValid && err != nil {
				t.Fatalf("ParseUsageGroupBy(%q) error = %v", tc.in, err)
			}
			if !tc.wantValid && err == nil {
				t.Fatalf("ParseUsageGroupBy(%q) = nil error, want a rejection", tc.in)
			}
		})
	}
}

// TestUsageGranularity_Set covers the bucket widths and the interval literal
// each one contributes to the statement.
func TestUsageGranularity_Set(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantValid  bool
		wantBucket string
	}{
		{"hour", "hour", true, "1 hour"},
		{"day", "day", true, "1 day"},
		{"empty is not a granularity", "", false, "1 hour"},
		{"minute is not offered", "minute", false, "1 hour"},
		{"uppercase", "HOUR", false, "1 hour"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			granularity := UsageGranularity(tc.in)
			if got := granularity.IsValid(); got != tc.wantValid {
				t.Fatalf("IsValid(%q) = %v, want %v", tc.in, got, tc.wantValid)
			}
			if got := granularity.Interval(); got != tc.wantBucket {
				t.Fatalf("Interval(%q) = %q, want %q", tc.in, got, tc.wantBucket)
			}
		})
	}
}

// TestUsageTotals_ErrorRate covers the derived rate at the boundaries: an empty
// range, an all-failed range, an all-succeeded range, and a fractional one.
func TestUsageTotals_ErrorRate(t *testing.T) {
	cases := []struct {
		name     string
		requests int64
		errors   int64
		want     string
	}{
		{"no requests", 0, 0, "0.0000"},
		{"no failures", 100, 0, "0.0000"},
		{"all failed", 7, 7, "1.0000"},
		{"one in three", 3, 1, "0.3333"},
		{"one in eight", 8, 1, "0.1250"},
		{"extreme all failed", 1_000_000, 1_000_000, "1.0000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			totals := UsageTotals{Requests: tc.requests, ErrorCount: tc.errors}
			if got := totals.ErrorRate(); got != tc.want {
				t.Fatalf("ErrorRate = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestUsageTotals_Add covers the arithmetic a group breakdown is totalled with,
// which is the same rule whether there are zero, one, or many groups.
func TestUsageTotals_Add(t *testing.T) {
	cases := []struct {
		name  string
		left  UsageTotals
		right UsageTotals
		want  UsageTotals
	}{
		{
			name:  "zero plus zero stays zero",
			left:  UsageTotals{},
			right: UsageTotals{},
			want:  UsageTotals{CostUSD: "0.00000000"},
		},
		{
			name:  "costs sum exactly",
			left:  UsageTotals{Requests: 2, TokensIn: 10, CostUSD: "0.00420000"},
			right: UsageTotals{Requests: 1, TokensOut: 5, CostUSD: "0.00000001"},
			want:  UsageTotals{Requests: 3, TokensIn: 10, TokensOut: 5, CostUSD: "0.00420001"},
		},
		{
			// The sum renders at the column scale, so an unset cost becomes the
			// canonical zero rather than an empty string: every totals block a
			// client sees is then the same shape whether or not it summed.
			name:  "counter fields add and the cost is canonicalized",
			left:  UsageTotals{TokensCacheRead: 7, TokensCacheWrite: 3, LatencyMS: 120, ErrorCount: 1},
			right: UsageTotals{TokensCacheRead: 2, TokensCacheWrite: 9, LatencyMS: 80, ErrorCount: 2},
			want:  UsageTotals{TokensCacheRead: 9, TokensCacheWrite: 12, CostUSD: "0.00000000", LatencyMS: 200, ErrorCount: 3},
		},
		{
			name:  "percentiles keep the left because they are not additive",
			left:  UsageTotals{LatencyP50MS: 40, LatencyP95MS: 90},
			right: UsageTotals{LatencyP50MS: 10, LatencyP95MS: 20},
			want:  UsageTotals{CostUSD: "0.00000000", LatencyP50MS: 40, LatencyP95MS: 90},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.left.Add(tc.right)
			if got != tc.want {
				t.Fatalf("Add = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestSumCosts covers the cost summation at the empty and malformed boundaries.
func TestSumCosts(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{"two zeros", "0", "0", "0.00000000"},
		{"empty is zero", "", "", "0.00000000"},
		{"exact", "0.0042", "0.0008", "0.00500000"},
		{"malformed operand is zero", "nonsense", "0.5", "0.50000000"},
		{"large magnitudes", "99999999.99999999", "0.00000001", "100000000.00000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SumCosts(tc.a, tc.b); got != tc.want {
				t.Fatalf("SumCosts(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestNewUsageFilter covers the default window, the explicit range, and the
// zero-record window a summary must still answer for.
func TestNewUsageFilter(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	early := now.Add(-72 * time.Hour)
	late := now.Add(-1 * time.Hour)

	cases := []struct {
		name     string
		in       UsageFilterInput
		wantFrom time.Time
		wantTo   time.Time
		wantErr  bool
	}{
		{"no range uses the default window", UsageFilterInput{}, now.Add(-DefaultUsageWindow), now, false},
		{"explicit range is kept", UsageFilterInput{From: &early, To: &late}, early, late, false},
		{"only from still bounds the end at now", UsageFilterInput{From: &early}, early, now, false},
		{"only to bounds the start before it", UsageFilterInput{To: &late}, late.Add(-DefaultUsageWindow), late, false},
		{"inverted range is rejected", UsageFilterInput{From: &late, To: &early}, time.Time{}, time.Time{}, true},
		{"equal bounds are a zero-length window, not an error", UsageFilterInput{From: &late, To: &late}, late, late, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewUsageFilter(tc.in, now)
			if err := filter.Validate(); tc.wantErr {
				if err == nil {
					t.Fatalf("Validate = nil, want an error for %+v", tc.in)
				}
				return
			} else if err != nil {
				t.Fatalf("Validate error = %v", err)
			}
			if !filter.From.Equal(tc.wantFrom) {
				t.Fatalf("From = %s, want %s", filter.From, tc.wantFrom)
			}
			if !filter.To.Equal(tc.wantTo) {
				t.Fatalf("To = %s, want %s", filter.To, tc.wantTo)
			}
		})
	}
}
