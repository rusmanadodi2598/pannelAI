// Package domain holds entities, value objects, and the ubiquitous language
//
// @file      internal/domain/combo_strategy_test.go
// @for       Table-driven tests for the pure round-robin rotation. (first half; split at the AGENTS.md §1.1 line limit).
// @uses      testing.
// @reason    SPEC-API-001 §7.7 ports combo.js's rotation, and that rule is the
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

// TestComboStrategy_NextOrder pins the rotation over the states a running
// gateway actually visits: the leading model advances only once the sticky
// limit is reached, and the strategies that do not rotate never move.
func TestComboStrategy_NextOrder(t *testing.T) {
	models := []string{"a/one", "b/two", "c/three"}
	cases := []struct {
		name     string
		strategy ComboStrategy
		models   []string
		sticky   int
		state    RotationState
		want     []string
		wantNext RotationState
	}{
		{
			name:     "round_robin holds the lead until the sticky limit",
			strategy: ComboRoundRobin, models: models, sticky: 2, state: RotationState{},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 0, Uses: 1},
		},
		{
			name:     "round_robin advances once the sticky limit is reached",
			strategy: ComboRoundRobin, models: models, sticky: 2, state: RotationState{Index: 0, Uses: 1},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 1},
		},
		{
			name:     "round_robin starts the next model from the rotated index",
			strategy: ComboRoundRobin, models: models, sticky: 1, state: RotationState{Index: 1},
			want: []string{"b/two", "c/three", "a/one"}, wantNext: RotationState{Index: 2},
		},
		{
			name:     "round_robin wraps back to the first model",
			strategy: ComboRoundRobin, models: models, sticky: 1, state: RotationState{Index: 2},
			want: []string{"c/three", "a/one", "b/two"}, wantNext: RotationState{Index: 0},
		},
		{
			name:     "round_robin keeps a large sticky limit on one model",
			strategy: ComboRoundRobin, models: models, sticky: 1000, state: RotationState{Index: 2, Uses: 500},
			want: []string{"c/three", "a/one", "b/two"}, wantNext: RotationState{Index: 2, Uses: 501},
		},
		{
			name:     "a single model never advances",
			strategy: ComboRoundRobin, models: []string{"a/one"}, sticky: 1, state: RotationState{Uses: 3},
			want: []string{"a/one"}, wantNext: RotationState{Uses: 3},
		},
		{
			name:     "an empty list is returned as an empty list",
			strategy: ComboRoundRobin, models: []string{}, sticky: 3, state: RotationState{},
			want: []string{}, wantNext: RotationState{},
		},
		{
			name:     "fallback keeps the stored order whatever the state",
			strategy: ComboFallback, models: models, sticky: 3, state: RotationState{Index: 2, Uses: 9},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 2, Uses: 9},
		},
		{
			name:     "fusion keeps the stored order whatever the state",
			strategy: ComboFusion, models: models, sticky: 3, state: RotationState{Index: 1},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 1},
		},
		{
			name:     "a zero sticky limit is floored to one",
			strategy: ComboRoundRobin, models: models, sticky: 0, state: RotationState{},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 1},
		},
		{
			name:     "a negative sticky limit is floored to one",
			strategy: ComboRoundRobin, models: models, sticky: -5, state: RotationState{},
			want: []string{"a/one", "b/two", "c/three"}, wantNext: RotationState{Index: 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, next := tc.strategy.NextOrder(tc.models, tc.sticky, tc.state)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NextOrder() order = %v, want %v", got, tc.want)
			}
			if next != tc.wantNext {
				t.Fatalf("NextOrder() state = %+v, want %+v", next, tc.wantNext)
			}
		})
	}
}

// TestComboStrategy_NextOrderDoesNotMutateInput pins the copy: the caller's
// slice is what the repository loaded, and a rotation that reordered it in place
// would corrupt an aggregate the caller still holds.
func TestComboStrategy_NextOrderDoesNotMutateInput(t *testing.T) {
	models := []string{"a/one", "b/two", "c/three"}
	original := append([]string(nil), models...)

	order, _ := ComboRoundRobin.NextOrder(models, 1, RotationState{Index: 2})
	if !reflect.DeepEqual(models, original) {
		t.Fatalf("NextOrder() mutated its input: %v, want %v", models, original)
	}
	if reflect.DeepEqual(order, models) {
		t.Fatalf("NextOrder() returned the stored order for a rotated state: %v", order)
	}
}

// TestRotationRequestIndex_MatchesNextOrder proves the Redis path and the pure
// path agree: the store persists only a request counter, so the index it derives
// must be exactly the index iterating NextOrder would arrive at.
func TestRotationRequestIndex_MatchesNextOrder(t *testing.T) {
	models := []string{"a/one", "b/two", "c/three"}
	for _, sticky := range []int{1, 2, 3, 5, 100} {
		for _, length := range []int{0, 1, 2, 3, 7} {
			state := RotationState{}
			for request := range 40 {
				want := models[:min(length, len(models))]
				order, next := ComboRoundRobin.NextOrder(want, sticky, state)
				state = next
				got := RotationRequestIndex(request, sticky, len(want))
				if len(want) < 2 {
					if got != 0 || !reflect.DeepEqual(order, want) {
						t.Fatalf("sticky=%d length=%d request=%d: short list must not rotate (got %v)", sticky, length, request, order)
					}
					continue
				}
				if order[0] != want[got] {
					t.Fatalf("sticky=%d length=%d request=%d: NextOrder led with %q, RotationRequestIndex chose %d (%q)",
						sticky, length, request, order[0], got, want[got])
				}
			}
		}
	}
}

// TestRotationRequestIndex_Boundaries covers the degenerate arguments directly.
func TestRotationRequestIndex_Boundaries(t *testing.T) {
	cases := []struct {
		name     string
		requests int
		sticky   int
		length   int
		want     int
	}{
		{"the first request with no stickiness", 0, 1, 3, 0},
		{"the last sticky request stays on the model", 2, 3, 3, 0},
		{"the next sticky window moves on", 3, 3, 3, 1},
		{"a full cycle wraps to the first model", 9, 3, 3, 0},
		{"an extreme request count wraps", 1000000, 3, 3, 0},
		{"a zero sticky limit is floored to one", 2, 0, 3, 2},
		{"a negative sticky limit is floored to one", 2, -4, 3, 2},
		{"a single model always leads", 7, 2, 1, 0},
		{"an empty list reports index zero", 7, 2, 0, 0},
		{"a negative request count is floored", -3, 2, 3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RotationRequestIndex(tc.requests, tc.sticky, tc.length); got != tc.want {
				t.Fatalf("RotationRequestIndex(%d, %d, %d) = %d, want %d",
					tc.requests, tc.sticky, tc.length, got, tc.want)
			}
		})
	}
}

// TestRotationState_Normalized covers a stored index that outlives the list it
// addressed: a combo edited from three models to two must not panic or skip.
func TestRotationState_Normalized(t *testing.T) {
	cases := []struct {
		name   string
		state  RotationState
		length int
		want   RotationState
	}{
		{"an index inside the list is kept", RotationState{Index: 1, Uses: 2}, 3, RotationState{Index: 1, Uses: 2}},
		{"an index past the end wraps", RotationState{Index: 5, Uses: 2}, 3, RotationState{Index: 2, Uses: 2}},
		{"a negative index wraps forward", RotationState{Index: -1, Uses: 0}, 3, RotationState{Index: 2}},
		{"a negative use count is floored", RotationState{Index: 0, Uses: -4}, 3, RotationState{Index: 0}},
		{"a zero-length list collapses the state", RotationState{Index: 7, Uses: 7}, 0, RotationState{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.Normalized(tc.length); got != tc.want {
				t.Fatalf("Normalized(%d) = %+v, want %+v", tc.length, got, tc.want)
			}
		})
	}
}
