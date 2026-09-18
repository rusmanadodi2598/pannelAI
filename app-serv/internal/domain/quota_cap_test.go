// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota_cap_test.go
// @for       Table-driven tests for the budget cap and the rule the router
//
//	skips an exhausted endpoint on.
//
// @uses      testing, time.
// @reason    AGENTS.md §2.1 and §2.4 require the cap rule to be pinned: whether
//
//	a cap binds decides if an endpoint is skipped, and the boundary
//	where usage equals the cap is the case an off-by-one gets wrong.
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

// TestQuotaCap_Exhausted covers the two-cap rule, including which cap binds
// when both are set and the uncapped case.
func TestQuotaCap_Exhausted(t *testing.T) {
	tenTokens := int64(1_000_000)
	zeroTokens := int64(0)

	cases := []struct {
		name          string
		cap           QuotaCap
		monthlyCost   string
		monthlyTokens int64
		want          bool
	}{
		{"no caps is never exhausted", mustCap(t, "ep_1", nil, nil), "999.00000000", 10_000_000, false},
		{"tokens below the cap", mustCap(t, "ep_1", nil, &tenTokens), "0", 999_999, false},
		{"tokens exactly at the cap", mustCap(t, "ep_1", nil, &tenTokens), "0", 1_000_000, true},
		{"tokens over the cap", mustCap(t, "ep_1", nil, &tenTokens), "0", 1_000_001, true},
		{"zero token cap is exhausted immediately", mustCap(t, "ep_1", nil, &zeroTokens), "0", 0, true},
		{"cost below the cap", mustCap(t, "ep_1", decimalPtr(t, "10.00"), nil), "9.99999999", 0, false},
		{"cost exactly at the cap", mustCap(t, "ep_1", decimalPtr(t, "10.00"), nil), "10.00000000", 0, true},
		{"cost binds while tokens are under", mustCap(t, "ep_1", decimalPtr(t, "10.00"), &tenTokens), "10.5", 5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost, err := ParseDecimal(tc.monthlyCost)
			if err != nil {
				t.Fatalf("ParseDecimal(%q) error = %v", tc.monthlyCost, err)
			}
			if got := tc.cap.Exhausted(cost, tc.monthlyTokens); got != tc.want {
				t.Fatalf("Exhausted(%s, %d) = %v, want %v", tc.monthlyCost, tc.monthlyTokens, got, tc.want)
			}
		})
	}
}

// TestNewQuotaCap covers the cap validation, including the degenerate single
// zero-cost cap that would otherwise skip an endpoint after one cent.
func TestNewQuotaCap(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	zero := ZeroDecimal()
	zeroTokens := int64(0)
	tenTokens := int64(100)

	cases := []struct {
		name       string
		endpointID string
		cost       *Decimal
		tokens     *int64
		wantErr    bool
	}{
		{"token cap alone", "ep_1", nil, &tenTokens, false},
		{"cost cap alone", "ep_1", decimalPtr(t, "5.00"), nil, false},
		{"both caps", "ep_1", decimalPtr(t, "5.00"), &tenTokens, false},
		{"zero token cap is allowed", "ep_1", nil, &zeroTokens, false},
		{"clear both caps", "ep_1", nil, nil, false},
		{"zero cost cap alone is rejected", "ep_1", &zero, nil, true},
		{"missing endpoint", "", nil, &tenTokens, true},
		{"negative tokens", "ep_1", nil, ptrInt64(-1), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cap, err := NewQuotaCap(tc.endpointID, tc.cost, tc.tokens, now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewQuotaCap = nil error, want a rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewQuotaCap error = %v", err)
			}
			if _, ok := cap.MonthlyCostUSD(); ok != (tc.cost != nil) {
				t.Fatalf("cost presence = %v, want %v", ok, tc.cost != nil)
			}
			if _, ok := cap.MonthlyTokens(); ok != (tc.tokens != nil) {
				t.Fatalf("token presence = %v, want %v", ok, tc.tokens != nil)
			}
		})
	}
}
