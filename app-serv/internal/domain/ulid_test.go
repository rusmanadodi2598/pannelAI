// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/ulid_test.go
// @for       Property tests for ULID generation: length, alphabet, monotonicity.
// @uses      testing, time (standard library only).
// @reason    SPEC-API-001 §4 makes IDs ULID strings, and the monotonic guard is
//
//	the invariant that keeps same-millisecond writes sortable; a
//	table-driven + randomized test keeps the generator honest beyond
//	any single fixture (docs/RULLES/TDD.md §2.4, §2.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// TestNewULID_Table verifies the format invariants that every ID must satisfy.
func TestNewULID_Table(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
	}{
		{"unix epoch", time.UnixMilli(0).UTC()},
		{"one millisecond", time.UnixMilli(1).UTC()},
		{"typical present", time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)},
		{"max 48-bit timestamp", time.UnixMilli(1<<48 - 1).UTC()},
		{"past the 48-bit range", time.UnixMilli(1 << 50).UTC()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := NewULID(tc.now)
			if len(id) != ulidLen {
				t.Fatalf("len = %d, want %d", len(id), ulidLen)
			}
			for _, c := range id {
				if !strings.ContainsRune(ulidAlphabet, c) {
					t.Fatalf("id %q contains non-alphabet rune %q", id, c)
				}
			}
		})
	}
}

// TestNewULID_MonotonicInSameMillisecond checks that IDs issued in the same
// millisecond are still distinct and increasing, because the timestamp is
// clamped forward rather than relying on the random part colliding.
func TestNewULID_MonotonicInSameMillisecond(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	prev := ""
	for i := 0; i < 2000; i++ {
		id := NewULID(now)
		if id <= prev && i > 0 {
			t.Fatalf("non-monotonic id at i=%d: %q <= prev %q", i, id, prev)
		}
		prev = id
	}
}

// TestNewULID_AcrossTime verifies that chronological order is preserved as
// lexicographic order, using strictly increasing cumulative offsets so the
// precondition the ordering invariant relies on actually holds.
func TestNewULID_AcrossTime(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	cursor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := make([]string, 0, 256)
	for i := 0; i < 256; i++ {
		cursor = cursor.Add(time.Duration(1 + rng.Int64N(int64(24*time.Hour))))
		ids = append(ids, NewULID(cursor))
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("ids out of order at %d: %q <= %q", i, ids[i], ids[i-1])
		}
	}
}

// TestNewULID_Uniqueness issues many IDs and asserts no collisions.
func TestNewULID_Uniqueness(t *testing.T) {
	const n = 20000
	seen := make(map[string]struct{}, n)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		id := NewULID(now.Add(time.Duration(i) * time.Millisecond))
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = struct{}{}
	}
}
