// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/combo_rotation_test.go
// @for       The rotation index tests that keep the Redis path and the pure path in step.
// @uses      testing.
// @reason    The NextOrder table and the closed-form index tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"reflect"
	"testing"
)

// TestRotateRefs pins the permutation on its own: the Redis-backed store applies
// it to the index the Lua script chose, so the two must agree about what an
// index means.
func TestRotateRefs(t *testing.T) {
	refs := []string{"a", "b", "c", "d"}
	cases := []struct {
		name string
		n    int
		want []string
	}{
		{"zero rotates nothing", 0, []string{"a", "b", "c", "d"}},
		{"one moves the head to the end", 1, []string{"b", "c", "d", "a"}},
		{"a full turn is the identity", 4, []string{"a", "b", "c", "d"}},
		{"an index past the end wraps", 6, []string{"c", "d", "a", "b"}},
		{"a negative index wraps forward", -1, []string{"d", "a", "b", "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RotateRefs(refs, tc.n); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RotateRefs(%d) = %v, want %v", tc.n, got, tc.want)
			}
		})
	}
	if got := RotateRefs(nil, 3); len(got) != 0 {
		t.Fatalf("RotateRefs(nil) = %v, want an empty list", got)
	}
}

// TestComboStrategy_Parse rejects every value outside the closed set the CHECK
// constraint mirrors.
func TestComboStrategy_Parse(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    ComboStrategy
		wantErr bool
	}{
		{"fallback", "fallback", ComboFallback, false},
		{"round_robin", "round_robin", ComboRoundRobin, false},
		{"fusion", "fusion", ComboFusion, false},
		{"surrounding whitespace is trimmed", "  fallback ", ComboFallback, false},
		{"empty", "", "", true},
		{"camel case is not accepted", "roundRobin", "", true},
		{"an unknown strategy", "sequential", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseComboStrategy(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseComboStrategy(%q) accepted an invalid strategy", tc.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseComboStrategy(%q) error = %v", tc.value, err)
			}
			if got != tc.want {
				t.Fatalf("ParseComboStrategy(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
