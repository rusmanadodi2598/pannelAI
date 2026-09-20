// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/quota_cap.go
// @for       The QuotaCap value object: an endpoint's optional monthly budget
//
//	ceiling and the rule that makes the router skip it.
//
// @uses      internal/domain (Decimal, AppError constructors), time.
// @reason    SPEC-API-001 §7.12 makes the cap the thing a router reads to skip
//
//	an exhausted endpoint, so the exhausted rule is a domain decision
//	rather than a comparison inside a query. Both fields are optional
//	because "no cap" and "a cap of zero" are different rules.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"strconv"
	"time"
)

// MaxQuotaMonthlyTokens is the largest monthly token cap a caller may set. The
// bigint column would accept far more, but a value past a trillion tokens a
// month is a typo (an extra digit), not a budget, and refusing it at validation
// keeps the refusal ahead of any driver limit.
const MaxQuotaMonthlyTokens int64 = 1_000_000_000_000

// MaxQuotaMonthlyCostUSD is the largest monthly cost cap a caller may set. The
// numeric(20,8) column would accept three more integer digits, but a value past
// a billion dollars a month is a typo, not a budget.
var MaxQuotaMonthlyCostUSD = DecimalFromInt64(1_000_000_000)

// ValidateQuotaCapValues is the one rule set for cap amounts, applied on every
// path a cap enters the system: the wire validator runs it after parsing the
// decimal string, and the domain constructor runs it as the last guard, so the
// two can never disagree about what a legal cap is (AGENTS.md §1.4).
func ValidateQuotaCapValues(monthlyCostUSD *Decimal, monthlyTokens *int64) error {
	if monthlyCostUSD != nil {
		if monthlyCostUSD.IsNegative() {
			return NewValidationError("monthly_cost_usd must not be negative")
		}
		if monthlyCostUSD.Cmp(MaxQuotaMonthlyCostUSD) > 0 {
			return NewValidationError("monthly_cost_usd must not exceed " + MaxQuotaMonthlyCostUSD.String())
		}
	}
	if monthlyTokens != nil {
		if *monthlyTokens < 0 {
			return NewValidationError("monthly_tokens must not be negative")
		}
		if *monthlyTokens > MaxQuotaMonthlyTokens {
			return NewValidationError("monthly_tokens must not exceed " + strconv.FormatInt(MaxQuotaMonthlyTokens, 10))
		}
	}
	if monthlyCostUSD != nil && monthlyCostUSD.IsZero() && monthlyTokens == nil {
		// A zero cost cap would make the router skip an endpoint after the
		// first fraction of a cent, which is never what a caller meant. It is
		// rejected rather than silently treated as "no cap".
		return NewValidationError("monthly_cost_usd must be greater than zero when it is the only cap")
	}
	return nil
}

// QuotaCap is an endpoint's optional budget ceiling. Both fields are pointers:
// a cap that was never set is not a cap of zero, and the router's rule differs
// between the two.
type QuotaCap struct {
	endpointID     string
	monthlyCostUSD *Decimal
	monthlyTokens  *int64
	updatedAt      time.Time
}

// NewQuotaCap builds a cap from validated values. A nil field clears that cap,
// which is how a client removes a budget without a separate delete route.
func NewQuotaCap(endpointID string, monthlyCostUSD *Decimal, monthlyTokens *int64, now time.Time) (QuotaCap, error) {
	if endpointID == "" {
		return QuotaCap{}, NewValidationError("endpoint_id is required")
	}
	if err := ValidateQuotaCapValues(monthlyCostUSD, monthlyTokens); err != nil {
		return QuotaCap{}, err
	}
	return QuotaCap{
		endpointID:     endpointID,
		monthlyCostUSD: monthlyCostUSD,
		monthlyTokens:  monthlyTokens,
		updatedAt:      now.UTC(),
	}, nil
}

// RehydrateQuotaCap rebuilds a stored row for the repository load path.
func RehydrateQuotaCap(endpointID string, monthlyCostUSD *Decimal, monthlyTokens *int64, updatedAt time.Time) QuotaCap {
	return QuotaCap{
		endpointID:     endpointID,
		monthlyCostUSD: monthlyCostUSD,
		monthlyTokens:  monthlyTokens,
		updatedAt:      updatedAt.UTC(),
	}
}

// Accessors expose the cap without allowing mutation.
func (c QuotaCap) EndpointID() string   { return c.endpointID }
func (c QuotaCap) UpdatedAt() time.Time { return c.updatedAt }
func (c QuotaCap) MonthlyCostUSD() (string, bool) {
	if c.monthlyCostUSD == nil {
		return "", false
	}
	return c.monthlyCostUSD.String(), true
}
func (c QuotaCap) MonthlyTokens() (int64, bool) {
	if c.monthlyTokens == nil {
		return 0, false
	}
	return *c.monthlyTokens, true
}

// Exhausted reports whether month-to-date usage has reached either cap. This is
// the rule the router reads to skip an endpoint (SPEC-API-001 §7.12).
func (c QuotaCap) Exhausted(monthlyCost Decimal, monthlyTokens int64) bool {
	if c.monthlyCostUSD != nil && monthlyCost.Cmp(*c.monthlyCostUSD) >= 0 {
		return true
	}
	if c.monthlyTokens != nil && monthlyTokens >= *c.monthlyTokens {
		return true
	}
	return false
}

// ErrQuotaCapNotFound is the sentinel a missing budget cap maps to.
var ErrQuotaCapNotFound = NewNotFoundError("quota cap not found")
