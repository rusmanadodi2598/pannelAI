// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_live_tracker_test.go
// @for       Table-driven tests for the in-flight tracker: the marker's write,
//
//	its release, and the failures that must not reach the request path.
//
// @uses      internal/domain, context, errors, testing, time.
// @reason    The tracker is what makes the drawing's active node mean "a request
//
//	is running now", and its whole contract is a pair of writes around
//	an interval: a marker that opens and never closes lights a node for
//	a request that ended, and a store failure that reached the caller
//	would fail a request over bookkeeping. Both rules are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestActiveRequestTracker_BeginAndRelease pins the tracker's contract: it
// records one marker per call, releases exactly that marker, and never lets a
// store failure reach the request path (the client already has its answer, and a
// drawing that is missing a node is better than a failed request).
func TestActiveRequestTracker_BeginAndRelease(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name          string
		providerID    string
		requestID     string
		storeErr      error
		wantStarted   int
		wantRemoved   int
		wantReleaseFn bool
	}{
		{name: "a call is recorded and released", providerID: "openai", requestID: "req_1", wantStarted: 1, wantRemoved: 1},
		{name: "a call with no provider records nothing", providerID: "", requestID: "req_2", wantStarted: 0, wantRemoved: 0},
		{name: "a store failure is swallowed", providerID: "openai", requestID: "req_3", storeErr: errors.New("redis is down"), wantStarted: 1, wantRemoved: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active := &liveActiveDouble{err: tc.storeErr}
			tracker := NewActiveRequestTracker(active, func(context.Context) string { return tc.requestID }, nil)
			tracker.SetClock(func() time.Time { return now })

			release := tracker.Begin(context.Background(), tc.providerID, "ep_1", "gpt-4o")
			if release == nil {
				t.Fatal("Begin() returned no release function, so the marker would never be removed")
			}
			// Releasing twice is the caller's safety net: a defer beside an
			// explicit call must not be an error.
			release()
			release()

			if got := active.startedCount(); got != tc.wantStarted {
				t.Fatalf("markers started = %d, want %d", got, tc.wantStarted)
			}
			if got := active.removedCount(); got != tc.wantRemoved {
				t.Fatalf("markers removed = %d, want %d", got, tc.wantRemoved)
			}
			if tc.wantRemoved > 0 {
				marker := active.removed[0]
				if marker.ProviderID != tc.providerID {
					t.Errorf("the released marker names provider %q, want %q", marker.ProviderID, tc.providerID)
				}
				if marker.RequestID != tc.requestID {
					t.Errorf("the released marker carries request %q, want %q", marker.RequestID, tc.requestID)
				}
				if !marker.StartedAt.Equal(now) {
					t.Errorf("the released marker started at %v, want %v", marker.StartedAt, now)
				}
			}
		})
	}
}

// TestActiveRequestTracker_NilTrackerIsANoOp pins that an unwired tracker
// returns a usable release function, so the three planes that call it need no
// nil check of their own.
func TestActiveRequestTracker_NilTrackerIsANoOp(t *testing.T) {
	var tracker *ActiveRequestTracker
	release := tracker.Begin(context.Background(), "openai", "ep_1", "gpt-4o")
	if release == nil {
		t.Fatal("a nil tracker returned no release function")
	}
	release()
}

// TestActiveRequestTracker_MintsARequestIDWhenNoneIsPresent pins the fallback:
// a marker is still writable outside an HTTP request (a probe or a worker), and
// the id it carries is one the usage row for the same call will not match, which
// is why the router's id is preferred wherever one exists.
func TestActiveRequestTracker_MintsARequestIDWhenNoneIsPresent(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	active := &liveActiveDouble{}
	tracker := NewActiveRequestTracker(active, func(context.Context) string { return "" }, nil)
	tracker.SetClock(func() time.Time { return now })
	release := tracker.Begin(context.Background(), "openai", "ep_1", "gpt-4o")
	defer release()

	if active.startedCount() != 1 {
		t.Fatalf("markers started = %d, want 1", active.startedCount())
	}
	if marker := active.started[0]; marker.RequestID == "" {
		t.Fatal("the marker carries no request id, so the row it describes is unfindable")
	}
}
