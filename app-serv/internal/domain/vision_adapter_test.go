// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/vision_adapter_test.go
// @for       Table-driven tests for the vision adapter's validation and (first half; split at the AGENTS.md §1.1 line limit).
// @uses      testing, time, reflect.
// @reason    §7.8 validates every adapter model through a capability predicate,
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// visionRef is a test shorthand for a parsed reference.
func visionRef(t *testing.T, raw string) ModelRef {
	t.Helper()
	ref, err := ParseModelRef(raw)
	if err != nil {
		t.Fatalf("ParseModelRef(%q) error = %v", raw, err)
	}
	return ref
}

// TestNewVisionAdapter_CapabilitySeam pins both sides of the predicate seam:
// what the predicate accepts is stored, and what it rejects is a validation
// error naming the model.
func TestNewVisionAdapter_CapabilitySeam(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	capable := func(ref ModelRef) bool { return ref.ProviderID() == "openai" }
	cases := []struct {
		name       string
		enabled    bool
		roundRobin bool
		models     []string
		predicate  VisionCapabilityCheck
		wantErr    string
		wantModels []string
	}{
		{
			name: "an accepted model is stored", enabled: true, models: []string{"openai/gpt-4o"},
			predicate: capable, wantModels: []string{"openai/gpt-4o"},
		},
		{
			name: "a rejected model is refused by name", enabled: true, models: []string{"anthropic/claude"},
			predicate: capable, wantErr: "not vision-capable: anthropic/claude",
		},
		{
			name: "one rejected model refuses the whole list", enabled: true,
			models: []string{"openai/gpt-4o", "anthropic/claude"}, predicate: capable,
			wantErr: "not vision-capable: anthropic/claude",
		},
		{
			name: "a nil predicate accepts nothing", enabled: true, models: []string{"openai/gpt-4o"},
			predicate: nil, wantErr: "not vision-capable",
		},
		{
			name: "duplicates collapse", enabled: true,
			models: []string{"openai/gpt-4o", "openai/gpt-4o"}, predicate: capable,
			wantModels: []string{"openai/gpt-4o"},
		},
		{
			name: "an empty list is allowed while disabled", enabled: false, models: nil,
			predicate: capable, wantModels: []string{},
		},
		{
			name: "a zero reference is refused", enabled: true, models: []string{""},
			predicate: func(ModelRef) bool { return true }, wantErr: "must be provider/model references",
		},
		{
			name: "a disabled adapter still validates its models", enabled: false, models: []string{"anthropic/claude"},
			predicate: capable, wantErr: "not vision-capable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refs := make([]ModelRef, 0, len(tc.models))
			for _, model := range tc.models {
				if model == "" {
					refs = append(refs, ModelRef{})
					continue
				}
				refs = append(refs, visionRef(t, model))
			}
			adapter, err := NewVisionAdapter(tc.enabled, tc.roundRobin, refs, tc.predicate, now)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewVisionAdapter() accepted %q", tc.name)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewVisionAdapter() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewVisionAdapter() error = %v", err)
			}
			if !reflect.DeepEqual(adapter.ModelStrings(), tc.wantModels) {
				t.Fatalf("ModelStrings() = %v, want %v", adapter.ModelStrings(), tc.wantModels)
			}
			if !adapter.UpdatedAt().Equal(now) {
				t.Fatalf("UpdatedAt() = %v, want %v", adapter.UpdatedAt(), now)
			}
			if adapter.Enabled() != tc.enabled || adapter.RoundRobin() != tc.roundRobin {
				t.Fatalf("flags = %t/%t, want %t/%t", adapter.Enabled(), adapter.RoundRobin(), tc.enabled, tc.roundRobin)
			}
		})
	}
}

// TestVisionAdapter_Active pins what makes the adapter fire: enabled with at
// least one model. §7.8 renders the other combination as a warning, not as a
// routing state.
func TestVisionAdapter_Active(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	accept := func(ModelRef) bool { return true }
	cases := []struct {
		name    string
		enabled bool
		models  []string
		want    bool
	}{
		{name: "enabled with a model", enabled: true, models: []string{"openai/gpt-4o"}, want: true},
		{name: "enabled with no model", enabled: true, models: nil, want: false},
		{name: "disabled with a model", enabled: false, models: []string{"openai/gpt-4o"}, want: false},
		{name: "disabled with no model", enabled: false, models: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refs := make([]ModelRef, 0, len(tc.models))
			for _, model := range tc.models {
				refs = append(refs, visionRef(t, model))
			}
			adapter, err := NewVisionAdapter(tc.enabled, true, refs, accept, now)
			if err != nil {
				t.Fatalf("NewVisionAdapter() error = %v", err)
			}
			if adapter.Active() != tc.want {
				t.Fatalf("Active() = %t, want %t", adapter.Active(), tc.want)
			}
		})
	}
}

// TestVisionAdapter_NextOrder proves the adapter rotates with the same rule the
// combo round-robin uses, and keeps the configured order when it does not
// rotate.
func TestVisionAdapter_NextOrder(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	accept := func(ModelRef) bool { return true }
	models := []ModelRef{visionRef(t, "a/one"), visionRef(t, "b/two"), visionRef(t, "c/three")}
	cases := []struct {
		name       string
		roundRobin bool
		state      RotationState
		want       []string
		wantNext   RotationState
	}{
		{
			name: "round_robin rotates from the stored index", roundRobin: true, state: RotationState{Index: 1},
			want: []string{"b/two", "c/three", "a/one"}, wantNext: RotationState{Index: 2},
		},
		{
			name: "round_robin alternates on every request", roundRobin: true, state: RotationState{Index: 2},
			want: []string{"c/three", "a/one", "b/two"}, wantNext: RotationState{Index: 0},
		},
		{
			name: "a static adapter keeps the configured order", roundRobin: false, state: RotationState{Index: 2},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := NewVisionAdapter(true, tc.roundRobin, models, accept, now)
			if err != nil {
				t.Fatalf("NewVisionAdapter() error = %v", err)
			}
			order, next := adapter.NextOrder(tc.state)
			if !reflect.DeepEqual(order, tc.want) {
				t.Fatalf("NextOrder() = %v, want %v", order, tc.want)
			}
			if next != tc.wantNext {
				t.Fatalf("NextOrder() state = %+v, want %+v", next, tc.wantNext)
			}
		})
	}
}

// TestVisionAdapter_SingleModelKeepsOrder covers the boundary where a rotation
// has nothing to distribute.
func TestVisionAdapter_SingleModelKeepsOrder(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	adapter, err := NewVisionAdapter(true, true, []ModelRef{visionRef(t, "a/one")},
		func(ModelRef) bool { return true }, now)
	if err != nil {
		t.Fatalf("NewVisionAdapter() error = %v", err)
	}
	for _, state := range []RotationState{{}, {Index: 0, Uses: 5}, {Index: 9}} {
		order, next := adapter.NextOrder(state)
		if !reflect.DeepEqual(order, []string{"a/one"}) {
			t.Fatalf("NextOrder(%+v) = %v, want the single model", state, order)
		}
		if next != state {
			t.Fatalf("NextOrder(%+v) state = %+v, want it unchanged", state, next)
		}
	}
}
