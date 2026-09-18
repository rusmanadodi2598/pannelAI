// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/vision_adapter_order_test.go
// @for       The vision adapter's rotation, identity, and default-state tests.
// @uses      testing, time, reflect.
// @reason    The validation tests and the rotation tests are separate groups; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"testing"
	"time"
)

// TestVisionAdapter_Includes recognises a model the adapter would have routed
// to, which is how a response identity is stripped back (§7.8).
func TestVisionAdapter_Includes(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	adapter, err := NewVisionAdapter(true, false, []ModelRef{visionRef(t, "openai/gpt-4o")},
		func(ModelRef) bool { return true }, now)
	if err != nil {
		t.Fatalf("NewVisionAdapter() error = %v", err)
	}
	cases := []struct {
		name string
		ref  string
		want bool
	}{
		{"a configured model", "openai/gpt-4o", true},
		{"a configured model with surrounding space", " openai/gpt-4o ", true},
		{"another model", "anthropic/claude", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := adapter.Includes(tc.ref); got != tc.want {
				t.Fatalf("Includes(%q) = %t, want %t", tc.ref, got, tc.want)
			}
		})
	}
}

// TestDefaultVisionAdapter pins the state a fresh install serves: disabled with
// an empty list, never a nil one, so the response serialises as [].
func TestDefaultVisionAdapter(t *testing.T) {
	adapter := DefaultVisionAdapter()
	if adapter.Enabled() || adapter.RoundRobin() || adapter.Active() {
		t.Fatalf("DefaultVisionAdapter() = enabled=%t roundRobin=%t active=%t, want all false",
			adapter.Enabled(), adapter.RoundRobin(), adapter.Active())
	}
	if adapter.Models() == nil || len(adapter.Models()) != 0 {
		t.Fatalf("DefaultVisionAdapter().Models() = %v, want an empty non-nil list", adapter.Models())
	}
	if !adapter.UpdatedAt().IsZero() {
		t.Fatalf("DefaultVisionAdapter().UpdatedAt() = %v, want the zero instant", adapter.UpdatedAt())
	}
}

// TestVisionAdapter_ReplaceRequiresTheSameEvidence proves a later PUT cannot
// bypass the predicate a constructor applies.
func TestVisionAdapter_ReplaceRequiresTheSameEvidence(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	adapter := DefaultVisionAdapter()
	reject := func(ModelRef) bool { return false }
	err := adapter.Replace(true, true, []ModelRef{visionRef(t, "openai/gpt-4o")}, reject, now)
	if err == nil {
		t.Fatal("Replace() accepted a model the predicate rejects")
	}
	if adapter.Enabled() || len(adapter.Models()) != 0 {
		t.Fatalf("Replace() mutated the adapter while rejecting: %+v", adapter)
	}
}
