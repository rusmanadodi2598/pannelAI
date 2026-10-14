// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_rotation_test.go
// @for       Tests for the selection-time stamp: what IdleAt orders by, the fallback for a stored row that predates it, and a key the network refused still advancing.
// @uses      testing, time, internal/domain
// @reason    Ordering keys by their last success is what let a burst pile onto one credential and a key the network refused win every later selection forever. The stamp is the fact rotation actually needs, and a regression here silently concentrates spend on one account.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import (
	"testing"
	"time"
)

// stampableKey builds one key of an endpoint with the state a stored row carries.
func stampableKey(id string, priority int, used, selected *time.Time) UpstreamKey {
	return RehydrateUpstreamKey(id, "ep_1", "key-"+id, sealedFixture, "sk-…cret",
		priority, UpstreamKeyActive, used, "", 0,
		KeyRotationState{LastSelectedAt: selected}, nil, keyNow, keyNow)
}

// endpointWithKeys builds an account owning the given keys in stored order.
func endpointWithKeys(t *testing.T, keys ...UpstreamKey) UpstreamEndpoint {
	t.Helper()
	endpoint, err := NewUpstreamEndpoint("ep_1", "alpha", "primary", UpstreamAuthAPIKey, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	for _, key := range keys {
		endpoint.AttachKey(key)
	}
	return endpoint
}

// TestUpstreamKey_IdleAtFallsBackToSuccess pins the pre-migration row: a key stored
// before the selection column exists still orders by the best fact there is, and an
// unused key stays the one rotation prefers.
func TestUpstreamKey_IdleAtFallsBackToSuccess(t *testing.T) {
	used := keyNow.Add(-time.Hour)
	cases := []struct {
		name     string
		used     *time.Time
		selected *time.Time
		want     *time.Time
	}{
		{name: "never chosen and never served reads unused", used: nil, selected: nil},
		{name: "a stored row predating the stamp orders by success", used: &used, selected: nil, want: &used},
		{name: "the selection stamp outranks the success", used: &used,
			selected: ptrTo(keyNow), want: ptrTo(keyNow)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := stampableKey("uky_1", 1, tc.used, tc.selected)
			got := key.IdleAt()
			if tc.want == nil {
				if got != nil {
					t.Fatalf("IdleAt() = %v, want nil", got)
				}
				return
			}
			if got == nil || !got.Equal(*tc.want) {
				t.Fatalf("IdleAt() = %v, want %v", got, *tc.want)
			}
		})
	}
}

// TestUpstreamKey_MarkSelectedAdvancesTheWalk pins the defect the stamp exists for:
// a chosen key stops being the oldest the moment it is chosen, so the next selection
// answers a different one and a whole set is walked, not one key hammered.
func TestUpstreamKey_MarkSelectedAdvancesTheWalk(t *testing.T) {
	endpoint := endpointWithKeys(t,
		stampableKey("uky_a", 1, nil, nil),
		stampableKey("uky_b", 1, nil, nil),
		stampableKey("uky_c", 1, nil, nil),
	)

	// Identical priority and no history: the tiebreak alone decides the first pick,
	// so every selection made against a success-only stamp would answer the same key.
	seen := make([]string, 0, 3)
	for round := 1; round <= 3; round++ {
		picked, ok := endpoint.NextKey(keyNow)
		if !ok {
			t.Fatalf("round %d: no key available", round)
		}
		if _, err := endpoint.RecordKeySelected(picked.ID(), keyNow.Add(time.Duration(round)*time.Second)); err != nil {
			t.Fatalf("RecordKeySelected(%s) error = %v", picked.ID(), err)
		}
		seen = append(seen, picked.ID())
	}
	if seen[0] == seen[1] || seen[1] == seen[2] || seen[0] == seen[2] {
		t.Fatalf("the walk repeated a credential: %v, want three distinct keys", seen)
	}
	if endpoint.LastSelectedAt() == nil {
		t.Fatal("a keyed selection must stamp the account too: it was chosen at that moment")
	}
}

// TestUpstreamEndpoint_RecordKeySelected pins the aggregate boundary: the root owns
// the child's transition, an unknown id is refused, and the key that came back is
// the one the caller persists.
func TestUpstreamEndpoint_RecordKeySelected(t *testing.T) {
	endpoint := endpointWithKeys(t, stampableKey("uky_a", 1, nil, nil))

	stamped, err := endpoint.RecordKeySelected("uky_a", keyNow)
	if err != nil {
		t.Fatalf("RecordKeySelected() error = %v", err)
	}
	if stamped.ID() != "uky_a" || stamped.LastSelectedAt() == nil {
		t.Fatalf("stamped key = %q/%v, want uky_a carrying the stamp", stamped.ID(), stamped.LastSelectedAt())
	}
	if _, err := endpoint.RecordKeySelected("uky_missing", keyNow); err == nil {
		t.Fatal("RecordKeySelected(missing) = nil, want not found")
	}
	// Health must not be touched by a choice: the call it announces has not happened.
	if got := endpoint.Key("uky_a").Status(); got != UpstreamKeyActive {
		t.Fatalf("status after selection = %q, want active", got)
	}
	if got := endpoint.Key("uky_a").LastUsedAt(); got != nil {
		t.Fatalf("last_used_at after selection = %v, want untouched", got)
	}
}

// TestUpstreamEndpoint_MarkSelectedKeyless pins the account's own stamp, the only
// rotation state an OAuth or no-auth row can carry.
func TestUpstreamEndpoint_MarkSelectedKeyless(t *testing.T) {
	endpoint, err := NewUpstreamEndpoint("ep_oauth", "alpha", "account", UpstreamAuthOAuth, 1, keyNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	if endpoint.LastSelectedAt() != nil {
		t.Fatal("a fresh account has not been chosen yet")
	}
	endpoint.MarkSelected(keyNow)
	got := endpoint.LastSelectedAt()
	if got == nil || !got.Equal(keyNow) {
		t.Fatalf("LastSelectedAt() = %v, want the stamp", got)
	}
}

// TestUpstreamKey_CloneKeepsTheStamp pins that copying an aggregate carries its
// rotation state: the selection path copies candidates, and a stamp lost in a copy
// would make a chosen key look unused again.
func TestUpstreamKey_CloneKeepsTheStamp(t *testing.T) {
	stamped := keyNow.Add(-time.Minute)
	endpoint := endpointWithKeys(t, stampableKey("uky_a", 1, nil, &stamped))
	endpoint.MarkSelected(stamped)

	copied := endpoint.Clone()
	key := copied.Key("uky_a")
	if got := key.LastSelectedAt(); got == nil || !got.Equal(stamped) {
		t.Fatalf("cloned stamp = %v, want %v", got, stamped)
	}
	// Clone gives the copy its own key array; the account's own stamp travels by
	// value, and losing either would make a chosen credential look unused again.
	if got := copied.LastSelectedAt(); got == nil || !got.Equal(stamped) {
		t.Fatalf("cloned account stamp = %v, want %v", got, stamped)
	}
}

func ptrTo[T any](v T) *T { return &v }
