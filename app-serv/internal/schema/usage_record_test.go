// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage_record_test.go
// @for       Table-driven tests for the quota cap body validation: the one
//
//	rule set the wire applies before the service sees a cap.
//
// @uses      internal/domain, strings, testing.
// @reason    Draft 005 F5 found two validators that could disagree: the schema
//
//	checked parseability and sign while the domain also rejected a
//	degenerate zero-cost cap, so the error a client saw depended on
//	which path ran first. These cases pin the shared rule set
//	(domain.ValidateQuotaCapValues) at the wire boundary, including the
//	ceilings that keep a typo from reaching the driver.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-20
package schema

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestValidateQuotaCap covers the cap body's rule set at the wire boundary:
// legal caps pass on every path, and each refusal names the rule, so the wire
// validator and the domain constructor cannot drift apart (draft 005 F5).
func TestValidateQuotaCap(t *testing.T) {
	atTokenCeiling := domain.MaxQuotaMonthlyTokens
	pastTokenCeiling := domain.MaxQuotaMonthlyTokens + 1

	cases := []struct {
		name    string
		request QuotaCapRequest
		wantErr string
	}{
		{
			name:    "a cost and token cap together is valid",
			request: QuotaCapRequest{strPtr("5"), int64Ptr(1000)},
		},
		{
			name:    "clearing both caps is valid",
			request: QuotaCapRequest{},
		},
		{
			name:    "a zero token cap is valid",
			request: QuotaCapRequest{MonthlyTokens: int64Ptr(0)},
		},
		{
			name:    "a zero cost beside a token cap is valid",
			request: QuotaCapRequest{MonthlyCostUSD: strPtr("0"), MonthlyTokens: int64Ptr(10)},
		},
		{
			name:    "a cost at the ceiling is valid",
			request: QuotaCapRequest{MonthlyCostUSD: strPtr("1000000000")},
		},
		{
			name:    "tokens at the ceiling are valid",
			request: QuotaCapRequest{MonthlyTokens: int64Ptr(atTokenCeiling)},
		},
		{
			name:    "a zero cost with no token cap is rejected",
			request: QuotaCapRequest{MonthlyCostUSD: strPtr("0")},
			wantErr: "must be greater than zero when it is the only cap",
		},
		{
			name:    "a negative cost is rejected",
			request: QuotaCapRequest{MonthlyCostUSD: strPtr("-1")},
			wantErr: "monthly_cost_usd must not be negative",
		},
		{
			name:    "a cost past the ceiling is rejected",
			request: QuotaCapRequest{MonthlyCostUSD: strPtr("1000000000.00000001")},
			wantErr: "monthly_cost_usd must not exceed",
		},
		{
			name:    "a cost the decimal parser refuses is rejected",
			request: QuotaCapRequest{MonthlyCostUSD: strPtr("five dollars")},
			wantErr: "VALIDATION_ERROR",
		},
		{
			name:    "negative tokens are rejected",
			request: QuotaCapRequest{MonthlyTokens: int64Ptr(-1)},
			wantErr: "monthly_tokens must not be negative",
		},
		{
			name:    "tokens past the ceiling are rejected",
			request: QuotaCapRequest{MonthlyTokens: int64Ptr(pastTokenCeiling)},
			wantErr: "monthly_tokens must not exceed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateQuotaCap(tc.request)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateQuotaCap() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateQuotaCap() = nil, want an error naming %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateQuotaCap() error = %v, want it to name %q", err, tc.wantErr)
			}
		})
	}
}

func strPtr(value string) *string { return &value }

func int64Ptr(value int64) *int64 { return &value }
