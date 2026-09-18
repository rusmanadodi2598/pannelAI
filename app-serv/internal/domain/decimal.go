// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/decimal.go
// @for       The decimal value object money and other exact amounts use.
// @uses      errors, fmt, math/big, strings (standard library only).
// @reason    SPEC-API-001 §4 requires cost to cross the wire as a decimal
//
//	string, never a float. A float cannot represent 0.1 exactly, so
//	summing a month of per-request costs in float64 would drift from
//	the sum a client computes from the strings it was shown. The
//	value is carried as big.Rat here and rendered as an exact decimal
//	string by the layers above.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import (
	"fmt"
	"math/big"
	"strings"
)

// decimalPlaces is the scale every rendered amount carries, matching the
// numeric(20, 8) column the usage and quota tables store (SPEC-API-001 §6).
const decimalPlaces = 8

// Decimal is an exact non-negative-scaled amount. The zero value is zero, so a
// struct field of this type needs no initialization.
type Decimal struct {
	value *big.Rat
}

// ParseDecimal reads a decimal string, rejecting anything that is not a finite
// base-10 number. The empty string parses to zero rather than failing: an
// absent cost is a zero cost in the schema this contract reads from.
func ParseDecimal(s string) (Decimal, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Decimal{}, nil
	}
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return Decimal{}, NewValidationError("value must be a decimal string")
	}
	return Decimal{value: rat}, nil
}

// NewDecimal builds a decimal from a big.Rat, which callers use when the value
// comes from a computed ratio rather than a parsed string.
func NewDecimal(rat *big.Rat) Decimal {
	if rat == nil {
		return Decimal{}
	}
	return Decimal{value: new(big.Rat).Set(rat)}
}

// ZeroDecimal is the additive identity, named so a caller reads intent.
func ZeroDecimal() Decimal { return Decimal{} }

// rat returns the underlying value, materializing zero on demand.
func (d Decimal) rat() *big.Rat {
	if d.value == nil {
		return new(big.Rat)
	}
	return d.value
}

// Add returns the exact sum.
func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Add(d.rat(), other.rat())}
}

// Sub returns the exact difference. It may be negative; callers that cannot
// accept that check IsNegative rather than getting a silently clamped zero.
func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Sub(d.rat(), other.rat())}
}

// MulInt scales by an integer, which is the aggregation step: a per-request
// cost multiplied by a count stays exact under this operation.
func (d Decimal) MulInt(factor int64) Decimal {
	return Decimal{value: new(big.Rat).Mul(d.rat(), new(big.Rat).SetInt64(factor))}
}

// IsNegative reports whether the amount is below zero.
func (d Decimal) IsNegative() bool { return d.rat().Sign() < 0 }

// IsZero reports whether the amount is exactly zero.
func (d Decimal) IsZero() bool { return d.rat().Sign() == 0 }

// Cmp orders two amounts: -1 below, 0 equal, 1 above.
func (d Decimal) Cmp(other Decimal) int { return d.rat().Cmp(other.rat()) }

// String renders the amount at the column's scale, so a value read from
// PostgreSQL round-trips unchanged and a client always sees the same shape.
func (d Decimal) String() string {
	return d.rat().FloatString(decimalPlaces)
}

// MarshalJSON renders the amount as a JSON string, never a JSON number, so a
// consumer parsing with a double-precision JSON parser cannot silently lose
// precision (SPEC-API-001 §4: money is a decimal string on the wire).
func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON reads a JSON string into the amount, accepting a bare number
// only so a value written by an older client still loads. The number path is
// deliberately narrow: a JSON number has already lost precision by the time
// encoding/json hands it over, so it is accepted for compatibility, not as the
// contract.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" || raw == `""` {
		d.value = nil
		return nil
	}
	unquoted := strings.Trim(raw, `"`)
	parsed, err := ParseDecimal(unquoted)
	if err != nil {
		return fmt.Errorf("decoding decimal: %w", err)
	}
	*d = parsed
	return nil
}

// DecimalFromInt64 builds an amount from an integer.
func DecimalFromInt64(v int64) Decimal {
	return Decimal{value: new(big.Rat).SetInt64(v)}
}
