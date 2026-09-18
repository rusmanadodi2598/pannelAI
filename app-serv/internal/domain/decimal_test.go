// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/decimal_test.go
// @for       Table-driven tests for the exact decimal value object.
// @uses      math/big, testing.
// @reason    AGENTS.md §2.1 and §2.4 require the arithmetic the cost figures
//
//	depend on to be pinned against boundary and extreme inputs. A float
//	would pass a single 0.1 test and drift under a month of sums, so
//	the cases below include the exact values a float cannot hold.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-18
package domain

import "testing"

// TestParseDecimal covers the accepted shapes and the rejected ones.
func TestParseDecimal(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"zero", "0", "0.00000000", false},
		{"empty is zero", "", "0.00000000", false},
		{"exact tenth", "0.1", "0.10000000", false},
		{"column scale", "0.00420000", "0.00420000", false},
		{"integer", "12", "12.00000000", false},
		{"negative", "-1.5", "-1.50000000", false},
		{"trimmed whitespace", "  2.25  ", "2.25000000", false},
		{"extreme scale", "0.00000001", "0.00000001", false},
		{"large", "123456789012.12345678", "123456789012.12345678", false},
		{"not a number", "abc", "", true},
		{"currency suffix", "1.5 usd", "", true},
		{"double dot", "1.2.3", "", true},
		{"infinity", "Inf", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDecimal(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDecimal(%q) = %q, want an error", tc.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDecimal(%q) error = %v", tc.in, err)
			}
			if got.String() != tc.want {
				t.Fatalf("ParseDecimal(%q) = %q, want %q", tc.in, got.String(), tc.want)
			}
		})
	}
}

// TestDecimal_AddIsExact pins the property a float cannot provide: the sum of
// ten exact tenths is exactly one, not 0.9999999999999999.
func TestDecimal_AddIsExact(t *testing.T) {
	cases := []struct {
		name  string
		terms []string
		want  string
	}{
		{"empty sum is zero", nil, "0.00000000"},
		{"single term", []string{"1.25"}, "1.25000000"},
		{"exact tenths", []string{"0.1", "0.1", "0.1", "0.1", "0.1", "0.1", "0.1", "0.1", "0.1", "0.1"}, "1.00000000"},
		{"mixed magnitudes", []string{"0.00000001", "99999999.99999999"}, "100000000.00000000"},
		{"cancels to zero", []string{"1.5", "-1.5"}, "0.00000000"},
		{"sum of extremes", []string{"0.0042", "0.0042", "0.0042", "0.0042"}, "0.01680000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sum := ZeroDecimal()
			for _, term := range tc.terms {
				parsed, err := ParseDecimal(term)
				if err != nil {
					t.Fatalf("ParseDecimal(%q) error = %v", term, err)
				}
				sum = sum.Add(parsed)
			}
			if sum.String() != tc.want {
				t.Fatalf("sum = %q, want %q", sum.String(), tc.want)
			}
		})
	}
}

// TestDecimal_OrderingAndSigns covers the comparisons the cap rule depends on,
// including the boundary where usage equals the cap.
func TestDecimal_OrderingAndSigns(t *testing.T) {
	cases := []struct {
		name     string
		left     string
		right    string
		wantCmp  int
		wantNeg  bool
		wantZero bool
	}{
		{"equal", "1.5", "1.5", 0, false, false},
		{"left lower", "1.49999999", "1.5", -1, false, false},
		{"left higher", "0.00000002", "0.00000001", 1, false, false},
		{"zero is zero", "0", "0", 0, false, true},
		{"negative below zero", "-0.00000001", "0", -1, true, false},
		{"empty equals zero", "", "0.00000000", 0, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			left, err := ParseDecimal(tc.left)
			if err != nil {
				t.Fatalf("left: %v", err)
			}
			right, err := ParseDecimal(tc.right)
			if err != nil {
				t.Fatalf("right: %v", err)
			}
			if got := left.Cmp(right); got != tc.wantCmp {
				t.Fatalf("Cmp = %d, want %d", got, tc.wantCmp)
			}
			if got := left.IsNegative(); got != tc.wantNeg {
				t.Fatalf("IsNegative = %v, want %v", got, tc.wantNeg)
			}
			if got := left.IsZero(); got != tc.wantZero {
				t.Fatalf("IsZero = %v, want %v", got, tc.wantZero)
			}
		})
	}
}

// TestDecimal_JSONRoundTrip pins the wire rule from SPEC-API-001 §4: a cost is
// a JSON string, never a JSON number.
func TestDecimal_JSONRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"string cost", `"0.00420000"`, `"0.00420000"`},
		{"zero", `"0"`, `"0.00000000"`},
		{"null is zero", `null`, `"0.00000000"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var value Decimal
			if err := value.UnmarshalJSON([]byte(tc.in)); err != nil {
				t.Fatalf("UnmarshalJSON(%s) error = %v", tc.in, err)
			}
			out, err := value.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON error = %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("round trip = %s, want %s", out, tc.want)
			}
		})
	}
}

// TestDecimal_MulIntAndSub covers the two operations the aggregation and the
// cap comparison use.
func TestDecimal_MulIntAndSub(t *testing.T) {
	cases := []struct {
		name       string
		base       string
		multiplier int64
		subtract   string
		want       string
	}{
		{"zero multiplier", "1.5", 0, "", "0.00000000"},
		{"identity", "0.0042", 1, "", "0.00420000"},
		{"scaled", "0.0042", 1000, "", "4.20000000"},
		{"exact subtraction", "1", 1, "0.1", "0.90000000"},
		{"goes negative", "0.5", 1, "1", "-0.50000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, err := ParseDecimal(tc.base)
			if err != nil {
				t.Fatalf("base: %v", err)
			}
			got := base.MulInt(tc.multiplier)
			if tc.subtract != "" {
				other, err := ParseDecimal(tc.subtract)
				if err != nil {
					t.Fatalf("subtract: %v", err)
				}
				got = got.Sub(other)
			}
			if got.String() != tc.want {
				t.Fatalf("result = %q, want %q", got.String(), tc.want)
			}
		})
	}
}
