// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_key_streaks_test.go
// @for       Tests for the per-class failure runs: which run a failure advances, what each ladder steps on, and what a success clears.
// @uses      testing, time, internal/domain
// @reason    One shared counter made a vendor's rate limiting escalate the credential ladder, so an account that was merely throttled was parked for an hour's worth of strikes it never took. The isolation is the rule SPEC-API-001 §7.5 now states, and it has to fail loudly rather than route traffic into a parked credential.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-14
package domain

import (
	"testing"
	"time"
)

// TestUpstreamKey_RecordFailureAdvancesOnlyItsOwnRun pins that a run of one class
// cannot lengthen another class's ladder.
func TestUpstreamKey_RecordFailureAdvancesOnlyItsOwnRun(t *testing.T) {
	key := newTestKey(t, 1)
	for range 3 {
		key.RecordFailure("throttled", KeyFailureRateLimit, DefaultKeyHealthPolicy(), keyNow)
	}
	if got := key.ConsecutiveRateLimit(); got != 3 {
		t.Fatalf("rate-limit run = %d, want 3", got)
	}
	if got := key.ConsecutiveAuth(); got != 0 {
		t.Fatalf("auth run = %d, want 0: a throttle is not a refused credential", got)
	}
	if got := key.ConsecutiveBilling(); got != 0 {
		t.Fatalf("billing run = %d, want 0", got)
	}
	// The third throttle parks at the third step of its own ladder: 2s doubled twice.
	if until := key.RateLimitedUntil(); until == nil || !until.Equal(keyNow.Add(8*time.Second)) {
		t.Fatalf("rate-limit park = %v, want the 8s third step", until)
	}

	// The credential failure that follows steps on a run of one, not four: the
	// shared counter would have parked it for eight minutes.
	key.RecordFailure("refused", KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow)
	if got := key.ConsecutiveAuth(); got != 1 {
		t.Fatalf("auth run = %d, want 1", got)
	}
	if until := key.RateLimitedUntil(); until == nil || !until.Equal(keyNow.Add(2*time.Minute)) {
		t.Fatalf("auth park = %v, want the 2m first step", until)
	}
	if got := key.ConsecutiveErrors(); got != 4 {
		t.Fatalf("total errors = %d, want 4: the display counter still sums every class", got)
	}
}

// TestUpstreamKey_RecordFailureParksTable walks each class's first step and its
// boundary cases, including the two classes that park nothing.
func TestUpstreamKey_RecordFailureParksTable(t *testing.T) {
	cases := []struct {
		name      string
		class     KeyFailureClass
		repeat    int
		wantRun   int
		wantUntil time.Duration
		wantClass string
	}{
		{name: "first auth strike", class: KeyFailureAuth, repeat: 1, wantRun: 1,
			wantUntil: 2 * time.Minute, wantClass: "auth"},
		{name: "fourth rate-limit strike", class: KeyFailureRateLimit, repeat: 4, wantRun: 4,
			wantUntil: 16 * time.Second, wantClass: "ratelimit"},
		{name: "second billing strike", class: KeyFailureBilling, repeat: 2, wantRun: 2,
			wantUntil: 30 * time.Minute, wantClass: "billing"},
		{name: "a hundred transient strikes stay flat", class: KeyFailureTransient, repeat: 100,
			// Transient has no run of its own: its window is flat, so it steps on
			// the total rather than on a ladder it never escalates.
			wantRun: 100, wantUntil: 30 * time.Second, wantClass: "transient"},
		{name: "a target-shaped failure parks nothing", class: KeyFailureTarget, repeat: 5,
			wantRun: 0, wantUntil: 0},
		{name: "a request-shaped failure parks nothing", class: KeyFailureRequest, repeat: 5,
			wantRun: 0, wantUntil: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t, 1)
			for range tc.repeat {
				key.RecordFailure("upstream said no", tc.class, DefaultKeyHealthPolicy(), keyNow)
			}
			if tc.wantUntil == 0 {
				if key.RateLimitedUntil() != nil {
					t.Fatalf("park = %v, want nothing set for a failure the credential caused",
						key.RateLimitedUntil())
				}
				return
			}
			until := key.RateLimitedUntil()
			if until == nil || !until.Equal(keyNow.Add(tc.wantUntil)) {
				t.Fatalf("park = %v, want %v", until, keyNow.Add(tc.wantUntil))
			}
			if got := key.RunFor(tc.class); got != tc.wantRun {
				t.Fatalf("run for %v = %d, want %d", tc.class, got, tc.wantRun)
			}
			if got := key.LastErrorClass(); got != tc.wantClass {
				t.Fatalf("last error class = %q, want %q", got, tc.wantClass)
			}
		})
	}
}

// TestUpstreamKey_RecordSuccessClearsEveryRun pins the reset: a credential that
// just answered is whole, and a stale run would park its next unrelated failure at
// the length of a problem it no longer has.
func TestUpstreamKey_RecordSuccessClearsEveryRun(t *testing.T) {
	key := newTestKey(t, 1)
	key.RecordFailure("throttled", KeyFailureRateLimit, DefaultKeyHealthPolicy(), keyNow)
	key.RecordFailure("refused", KeyFailureAuth, DefaultKeyHealthPolicy(), keyNow)
	key.RecordFailure("no credit", KeyFailureBilling, DefaultKeyHealthPolicy(), keyNow)

	key.RecordSuccess(keyNow.Add(time.Minute))

	for name, got := range map[string]int{
		"total":     key.ConsecutiveErrors(),
		"auth":      key.ConsecutiveAuth(),
		"ratelimit": key.ConsecutiveRateLimit(),
		"billing":   key.ConsecutiveBilling(),
	} {
		if got != 0 {
			t.Fatalf("%s run = %d, want 0 after a served call", name, got)
		}
	}
	if key.RateLimitedUntil() != nil {
		t.Fatal("a served call must release the park")
	}
	if got := key.Status(); got != UpstreamKeyActive {
		t.Fatalf("status = %q, want active", got)
	}
	if got := key.LastErrorClass(); got != "" {
		t.Fatalf("last error class = %q, want it cleared with the error", got)
	}
}

// TestKeyFailureClass_String pins the names the health statement compares against.
// They are storage values, not labels: a rename here moves a column's contents.
func TestKeyFailureClass_String(t *testing.T) {
	cases := []struct {
		class KeyFailureClass
		want  string
	}{
		{KeyFailureAuth, "auth"},
		{KeyFailureRateLimit, "ratelimit"},
		{KeyFailureBilling, "billing"},
		{KeyFailureTransient, "transient"},
		{KeyFailureRequest, "request"},
		{KeyFailureTarget, "target"},
		{KeyFailureClass(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.class.String(); got != tc.want {
			t.Errorf("class %d = %q, want %q", tc.class, got, tc.want)
		}
	}
}
