// Package schema tests dashboard authentication payload boundaries.
//
// @file      internal/schema/auth_test.go
// @for       Table-driven validation of password byte limits at the HTTP boundary.
// @uses      strings, testing, internal/schema.
// @reason    bcrypt accepts at most 72 bytes, so validation must protect the
//
//	service from multi-byte inputs that pass rune-based max checks.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-17
package schema

import (
	"strings"
	"testing"
)

func TestValidatePasswordBytesTable(t *testing.T) {
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "empty handled by struct validator", value: "", valid: true},
		{name: "ascii boundary", value: strings.Repeat("a", 72), valid: true},
		{name: "ascii above boundary", value: strings.Repeat("a", 73), valid: false},
		{name: "multibyte within bytes", value: "秘密-123", valid: true},
		{name: "multibyte above bytes", value: strings.Repeat("秘密", 25), valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasswordBytes(tc.value)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidatePasswordBytes() error=%v valid=%v", err, tc.valid)
			}
		})
	}
}
