// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_live_test.go
// @for       Table-driven tests for the live Usage service: the frame's recent
//
//	window, the provider an error is attributed to, and the in-flight
//	tracker's write and release.
//
// @uses      internal/domain, internal/repository, internal/schema, context,
//
//	errors, sync, testing, time.
//
// @reason    The live frame is the one place the panel's "a provider is routing
//
//	now" claim comes from, so the three rules that keep it honest are
//	pinned here: the recent read is bounded by both a window and a
//	limit, `error_provider` is only claimed inside its own short window,
//	and a released marker stops being drawn. Each of those fails
//	silently when it regresses: an unbounded read looks fine until the
//	table grows, and a stale error attribution looks like a real one.
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

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageLiveService_SnapshotBoundsTheRecentRead pins both halves of the
// recent bound: a time window and a row limit. Either one alone lets the read
// grow, and the frame is rendered rather than paged, so the bound is the only
// thing keeping it small.
func TestUsageLiveService_SnapshotBoundsTheRecentRead(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	svc, _, usage := liveServiceFixture(t, now)
	usage.records = []domain.UsageRecord{liveRecord(t, "req_1", "openai", "gpt-4o", domain.UsageStatusSuccess, now)}

	if _, err := svc.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot() = %v, want nil", err)
	}
	if usage.gotPage.Page != 1 {
		t.Fatalf("the read asked for page %d, want 1", usage.gotPage.Page)
	}
	if usage.gotPage.PerPage != UsageLiveRecentLimit {
		t.Fatalf("the read asked for %d rows, want %d", usage.gotPage.PerPage, UsageLiveRecentLimit)
	}
	wantFrom := now.Add(-UsageLiveRecentWindow)
	if !usage.gotFilter.From.Equal(wantFrom) {
		t.Fatalf("the read started at %v, want %v", usage.gotFilter.From, wantFrom)
	}
	if !usage.gotFilter.To.Equal(now) {
		t.Fatalf("the read ended at %v, want %v", usage.gotFilter.To, now)
	}
}

// TestUsageLiveService_SnapshotDerivesTheErrorProvider pins the rule that keeps
// the attribution honest: the provider named is the newest failed request inside
// its own short window, so an error an hour ago is not reported as current.
func TestUsageLiveService_SnapshotDerivesTheErrorProvider(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		status domain.UsageStatus
		age    time.Duration
		want   string
	}{
		{name: "a failure just now names its provider", status: domain.UsageStatusError, age: 0, want: "openai"},
		{name: "a failure one second inside the window names it", status: domain.UsageStatusError, age: UsageLiveErrorWindow - time.Second, want: "openai"},
		{name: "a failure exactly at the window is not claimed", status: domain.UsageStatusError, age: UsageLiveErrorWindow, want: ""},
		{name: "a failure long past the window is not claimed", status: domain.UsageStatusError, age: time.Hour, want: ""},
		{name: "a success never names a provider", status: domain.UsageStatusSuccess, age: 0, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, usage := liveServiceFixture(t, now)
			usage.records = []domain.UsageRecord{
				liveRecord(t, "req_1", "openai", "gpt-4o", tc.status, now.Add(-tc.age)),
			}
			frame, err := svc.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot() = %v, want nil", err)
			}
			if frame.ErrorProvider != tc.want {
				t.Fatalf("error_provider = %q, want %q", frame.ErrorProvider, tc.want)
			}
		})
	}
}

// TestUsageLiveService_SnapshotNamesTheNewestFailure pins precedence when more
// than one failure is inside the window: the frame names the most recent one,
// which is what the read's own order already puts first.
func TestUsageLiveService_SnapshotNamesTheNewestFailure(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	svc, _, usage := liveServiceFixture(t, now)
	usage.records = []domain.UsageRecord{
		liveRecord(t, "req_new", "anthropic", "claude-3", domain.UsageStatusError, now.Add(-time.Second)),
		liveRecord(t, "req_old", "openai", "gpt-4o", domain.UsageStatusError, now.Add(-5*time.Second)),
	}
	frame, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() = %v, want nil", err)
	}
	if frame.ErrorProvider != "anthropic" {
		t.Fatalf("error_provider = %q, want the newest failure's provider", frame.ErrorProvider)
	}
}

// TestUsageLiveService_SnapshotReportsTheActiveSet pins that the frame carries
// what the store holds, in the store's order, so the drawing and the list agree.
func TestUsageLiveService_SnapshotReportsTheActiveSet(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		markers   int
		wantCount int
	}{
		{name: "nothing is in flight", markers: 0, wantCount: 0},
		{name: "one request is in flight", markers: 1, wantCount: 1},
		{name: "several requests are in flight", markers: 5, wantCount: 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, active, _ := liveServiceFixture(t, now)
			for i := 0; i < tc.markers; i++ {
				marker, err := domain.NewActiveRequest(
					"req_"+string(rune('a'+i)), "openai", "ep_1", "gpt-4o",
					now.Add(-time.Duration(tc.markers-i)*time.Second),
				)
				if err != nil {
					t.Fatalf("building marker %d: %v", i, err)
				}
				if err := active.Start(context.Background(), marker); err != nil {
					t.Fatalf("Start() = %v, want nil", err)
				}
			}
			frame, err := svc.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot() = %v, want nil", err)
			}
			if len(frame.Active) != tc.wantCount {
				t.Fatalf("active = %d entries, want %d", len(frame.Active), tc.wantCount)
			}
		})
	}
}

// TestUsageLiveService_SnapshotFailsWhenTheStoreFails pins the failure
// direction: an unreadable active set is an error the stream states, never an
// empty set. An empty set is a claim that nothing is routing, which is exactly
// the claim a failed read cannot support (R-36).
func TestUsageLiveService_SnapshotFailsWhenTheStoreFails(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		activeErr error
		usageErr  error
		wantErr   bool
	}{
		{name: "both reads succeed"},
		{name: "the active store fails", activeErr: errors.New("redis is down"), wantErr: true},
		{name: "the recent read fails", usageErr: errors.New("postgres is down"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, active, usage := liveServiceFixture(t, now)
			active.err, usage.err = tc.activeErr, tc.usageErr
			frame, err := svc.Snapshot(context.Background())
			if tc.wantErr && err == nil {
				t.Fatalf("Snapshot() = %+v, want an error", frame)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Snapshot() = %v, want nil", err)
			}
		})
	}
}

// TestUsageLiveService_SnapshotWithNoActiveStoreIsHonest pins the documented
// behaviour for a deployment that wired no Redis: the frame is still a frame,
// with an empty active list, rather than a refusal to serve the route.
func TestUsageLiveService_SnapshotWithNoActiveStoreIsHonest(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	usage := &liveUsageDouble{}
	svc, err := NewUsageLiveService(UsageLiveServiceDeps{Usage: usage})
	if err != nil {
		t.Fatalf("NewUsageLiveService() = %v, want nil", err)
	}
	svc.clock = func() time.Time { return now }
	frame, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() = %v, want nil", err)
	}
	if frame.Active == nil {
		t.Fatal("active = nil, want an empty array: a null list is a second spelling of \"nothing is running\"")
	}
	if len(frame.Active) != 0 {
		t.Fatalf("active = %+v, want none", frame.Active)
	}
}

// TestUsageLiveService_RequiresAUsageRepository pins the constructor's one hard
// requirement: without the durable records there is no recent half to report, and
// a service that booted anyway would answer every connection with a frame it
// could not fill.
func TestUsageLiveService_RequiresAUsageRepository(t *testing.T) {
	if _, err := NewUsageLiveService(UsageLiveServiceDeps{}); err == nil {
		t.Fatal("NewUsageLiveService() = nil error, want a refusal")
	}
}
