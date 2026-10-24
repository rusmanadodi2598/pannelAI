// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/operator_window_test.go
// @for       The operator's disable window on both roots: when it holds a row out of rotation, when the clock releases it, and what a re-enable clears.
// @uses      fmt, testing, time.
// @reason    The window is the whole difference between a park that needs an operator and one that needs a scheduler, so the three cases that decide it are pinned at the instant they change: a deadline still counting, the deadline itself, and a disable with no deadline at all. Both roots are covered because rotation reads them separately and a rule enforced on one only is the asymmetry MarkUnhealthy was fixed for.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import (
	"fmt"
	"testing"
	"time"
)

var windowNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func windowEndpoint(t *testing.T) *UpstreamEndpoint {
	t.Helper()
	// No stored key is wanted here: the window is the subject, and an api_key account
	// with no keys would fail Available() for a reason that has nothing to do with it.
	endpoint, err := NewUpstreamEndpoint("ep_win", "alpha", "primary", UpstreamAuthNone, 1, windowNow)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	return &endpoint
}

func windowKey(t *testing.T) *UpstreamKey {
	t.Helper()
	key, err := NewUpstreamKey("k_win", "ep_win", "id", "sealed", "sk-…cret", 1, windowNow)
	if err != nil {
		t.Fatalf("NewUpstreamKey() error = %v", err)
	}
	return &key
}

func TestUpstreamEndpointDisableWindowHoldsThenReleases(t *testing.T) {
	cases := []struct {
		name      string
		until     time.Time
		readAt    time.Time
		wantAvail bool
		wantDone  bool
	}{
		{"a window still counting holds it out", windowNow.Add(time.Hour), windowNow.Add(time.Minute), false, false},
		{"the deadline instant itself releases it", windowNow.Add(time.Hour), windowNow.Add(time.Hour), true, true},
		{"a read past the deadline releases it", windowNow.Add(time.Hour), windowNow.Add(2 * time.Hour), true, true},
		{"a deadline already past when written is permanent", windowNow.Add(-time.Minute), windowNow.Add(30 * 24 * time.Hour), false, false},
		{"no window is permanent", time.Time{}, windowNow.Add(30 * 24 * time.Hour), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := windowEndpoint(t)
			endpoint.Disable(tc.until, windowNow)
			if endpoint.Status() != UpstreamEndpointDisabled {
				t.Fatalf("status = %q, want disabled", endpoint.Status())
			}
			if got := endpoint.Available(tc.readAt); got != tc.wantAvail {
				t.Fatalf("Available(%v) = %v, want %v", tc.readAt.Sub(windowNow), got, tc.wantAvail)
			}
			if got := endpoint.DisableWindowElapsed(tc.readAt); got != tc.wantDone {
				t.Fatalf("DisableWindowElapsed = %v, want %v", got, tc.wantDone)
			}
		})
	}
}

func TestUpstreamKeyDisableWindowHoldsThenReleases(t *testing.T) {
	cases := []struct {
		name      string
		until     time.Time
		readAt    time.Time
		wantAvail bool
	}{
		{"a window still counting holds it out", windowNow.Add(24 * time.Hour), windowNow, false},
		{"the deadline instant returns it to rotation", windowNow.Add(24 * time.Hour), windowNow.Add(24 * time.Hour), true},
		{"no window is permanent", time.Time{}, windowNow.Add(72 * time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := windowKey(t)
			key.Disable(tc.until, windowNow)
			if got := key.Available(tc.readAt); got != tc.wantAvail {
				t.Fatalf("Available(%v) = %v, want %v", tc.readAt.Sub(windowNow), got, tc.wantAvail)
			}
		})
	}
}

// TestDisableClearsTheHealthParkAndEnableClearsTheStreaks pins the trade D3 makes: an
// account switched off is not in a backoff, so the countdown goes, and a re-enabled one
// starts its breaker over rather than inheriting strikes it was never allowed to spend.
func TestDisableClearsTheHealthParkAndEnableClearsTheStreaks(t *testing.T) {
	parked := windowNow.Add(30 * time.Minute)
	endpoint := RehydrateUpstreamEndpoint("ep_win", "alpha", "primary", UpstreamAuthAPIKey, 1,
		UpstreamEndpointActive, nil, EndpointAccount{}, EndpointTestStatus{},
		&parked, nil, windowNow, windowNow, nil, ServiceAccountRef{}, EndpointParity{},
		OperatorState{})

	endpoint.Disable(windowNow.Add(time.Hour), windowNow)
	if endpoint.RateLimitedUntil() != nil {
		t.Fatal("a disabled account still carries a health countdown")
	}
	endpoint.Enable(windowNow)
	if endpoint.DisabledUntil() != nil {
		t.Fatal("re-enabling left a window behind to go stale")
	}
	if endpoint.Status() != UpstreamEndpointActive {
		t.Fatalf("status = %q, want active", endpoint.Status())
	}

	key := windowKey(t)
	key.RecordFailure("rate limited", KeyFailureRateLimit, DefaultKeyHealthPolicy(), windowNow)
	if key.ConsecutiveRateLimit() == 0 {
		t.Fatal("the staged failure did not raise the run")
	}
	key.Disable(windowNow.Add(time.Hour), windowNow)
	if key.RateLimitedUntil() != nil {
		t.Fatal("a disabled key still carries a health park")
	}
	key.Enable(windowNow)
	if got := key.ConsecutiveRateLimit(); got != 0 {
		t.Fatalf("rate-limit run after enable = %d, want the streak cleared", got)
	}
}

// TestPatchStatusLeavesNoWindowBehind pins the one asymmetry the window could otherwise
// keep: a PATCH moves the switch but states no deadline, so an earlier bulk park must not
// release the account again on its own schedule.
func TestPatchStatusLeavesNoWindowBehind(t *testing.T) {
	endpoint := windowEndpoint(t)
	endpoint.Disable(windowNow.Add(24*time.Hour), windowNow)
	if err := endpoint.Update("primary", 1, string(UpstreamEndpointActive), windowNow); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if endpoint.DisabledUntil() != nil {
		t.Fatal("the PATCH left the bulk window on the row")
	}

	key := windowKey(t)
	key.Disable(windowNow.Add(24*time.Hour), windowNow)
	if err := key.Transition(UpstreamKeyActive); err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if key.DisabledUntil() != nil {
		t.Fatal("the PATCH left the bulk window on the key")
	}
}

// TestRehydrateCarriesAnExpiredWindowIntoTheServingGate proves the load path, not just the
// method: a row parked by an earlier binary and read after its deadline must serve, which
// is what makes the absence of a sweeper safe rather than a bug waiting for traffic.
func TestRehydrateCarriesAnExpiredWindowIntoTheServingGate(t *testing.T) {
	deadline := windowNow.Add(-time.Second)
	endpoint := RehydrateUpstreamEndpoint("ep_win", "alpha", "primary", UpstreamAuthNone, 1,
		UpstreamEndpointDisabled, nil, EndpointAccount{}, EndpointTestStatus{},
		nil, nil, windowNow, windowNow, nil, ServiceAccountRef{}, EndpointParity{},
		OperatorState{Tags: []Tag{Tag("free-tier")}, DisabledUntil: &deadline})
	if !endpoint.Available(windowNow) {
		t.Fatal("an endpoint whose window elapsed is still held out of rotation")
	}
	if len(endpoint.Tags()) != 1 || endpoint.Tags()[0] != "free-tier" {
		t.Fatalf("tags = %v, want the stored set", endpoint.Tags())
	}

	key := RehydrateUpstreamKey("k_win", "ep_win", "primary", "sealed", "sk-…cret", 1,
		UpstreamKeyDisabled, nil, "", 0, KeyRotationState{}, nil, windowNow, windowNow,
		OperatorState{DisabledUntil: &deadline})
	if !key.Available(windowNow) {
		t.Fatal("a key whose window elapsed is still held out of rotation")
	}
}

func TestEndpointTagsRoundTripThroughTheAggregate(t *testing.T) {
	endpoint := windowEndpoint(t)

	added, err := endpoint.AddTag("Event_Week", windowNow)
	if err != nil || !added {
		t.Fatalf("AddTag() = (%v, %v), want the tag accepted", added, err)
	}
	again, err := endpoint.AddTag(" event_week ", windowNow)
	if err != nil {
		t.Fatalf("AddTag() error = %v", err)
	}
	if again {
		t.Fatal("a second spelling of one label added a duplicate")
	}
	updated := endpoint.UpdatedAt()

	removed, err := endpoint.RemoveTag("event_week", windowNow.Add(time.Minute))
	if err != nil || !removed {
		t.Fatalf("RemoveTag() = (%v, %v), want the tag gone", removed, err)
	}
	if !endpoint.UpdatedAt().After(updated) {
		t.Fatal("a tag change did not move updated_at, so the write would be lost to a conflict guard")
	}

	missing, err := endpoint.RemoveTag("never-held", windowNow)
	if err != nil {
		t.Fatalf("RemoveTag() error = %v", err)
	}
	if missing {
		t.Fatal("removing a tag the row never held reported a change")
	}

	for i := range MaxTagsPerRow {
		if _, err := endpoint.AddTag(fmt.Sprintf("fill-%02d", i), windowNow); err != nil {
			t.Fatalf("filling to the bound: %v", err)
		}
	}
	if len(endpoint.Tags()) != MaxTagsPerRow {
		t.Fatalf("tags = %d, want the set filled to the bound", len(endpoint.Tags()))
	}
	if _, err := endpoint.AddTag("one-past-the-bound", windowNow); err == nil {
		t.Fatal("a seventeenth tag was accepted")
	}
	if _, err := endpoint.AddTag("Bad Space", windowNow); err == nil {
		t.Fatal("a malformed tag was accepted")
	}
	endpoint.ClearTags(windowNow)
	if len(endpoint.Tags()) != 0 {
		t.Fatalf("tags = %v, want the set cleared", endpoint.Tags())
	}
}
