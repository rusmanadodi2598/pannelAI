// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_counter_test.go
// @for       Table-driven tests for the ingest half of the quota vertical: the
//
//	counter one served request advances (register G22).
//
// @uses      context, errors, internal/domain, log/slog, testing, time.
// @reason    F1 of docs/DRAFT/005-QUOTA-TRACKER-CLOSURE.md found the counter store
//
//	with no production caller, so every window read zero while a
//	flush worker drained a keyspace nothing filled. These tests pin the
//	three properties that keep it fixed: every accounting window is
//	advanced, the reset instant is in the future, and a write that fails
//	does not fail the request it was accounting for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubCounterStore records every add so a test can assert what was written
// without a Redis client.
type stubCounterStore struct {
	adds []counterAdd
	err  error
}

type counterAdd struct {
	endpointID string
	kind       domain.QuotaWindowKind
	units      int64
	resetsAt   time.Time
}

func (s *stubCounterStore) Add(_ context.Context, endpointID string, kind domain.QuotaWindowKind, units int64, resetsAt time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.adds = append(s.adds, counterAdd{endpointID, kind, units, resetsAt})
	return nil
}

func (s *stubCounterStore) Pending(context.Context, int) ([]domain.QuotaWindow, error) {
	return nil, nil
}

func (s *stubCounterStore) Clear(context.Context, []domain.QuotaWindow) error { return nil }

// quietLogger discards the counter's own log lines so a failing-write case does
// not print an error the test already asserts.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestQuotaCounter_RecordAdvancesEveryWindow is the F1 definition of done: one
// served request advances every window the schema stores, with a reset instant
// ahead of the clock, so the window the panel reads is what was served.
func TestQuotaCounter_RecordAdvancesEveryWindow(t *testing.T) {
	store := &stubCounterStore{}
	counter := NewQuotaCounter(store, quietLogger())
	now := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	counter.SetClock(func() time.Time { return now })

	cases := []struct {
		name       string
		endpointID string
		units      int64
		wantWrites int
	}{
		{"a served call advances every window", "ep_one", 150, 4},
		{"a single token is enough", "ep_two", 1, 4},
		{"a large spend is not clamped", "ep_three", 1 << 40, 4},
		{"no endpoint has nothing to key on", "", 10, 0},
		{"zero units is not a counter", "ep_four", 0, 0},
		{"negative units is not a counter", "ep_five", -5, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store.adds = nil
			counter.Record(context.Background(), tc.endpointID, tc.units)
			if len(store.adds) != tc.wantWrites {
				t.Fatalf("Record(%q, %d) wrote %d counters, want %d",
					tc.endpointID, tc.units, len(store.adds), tc.wantWrites)
			}
			if tc.wantWrites == 0 {
				return
			}
			seen := map[domain.QuotaWindowKind]bool{}
			for _, add := range store.adds {
				if add.endpointID != tc.endpointID {
					t.Fatalf("counter keyed to %q, want %q", add.endpointID, tc.endpointID)
				}
				if add.units != tc.units {
					t.Fatalf("counter advanced by %d, want %d", add.units, tc.units)
				}
				if !add.resetsAt.After(now) {
					t.Fatalf("window %q resets at %s, which is not after now %s",
						add.kind, add.resetsAt, now)
				}
				seen[add.kind] = true
			}
			for _, kind := range domain.AccountingKinds() {
				if !seen[kind] {
					t.Fatalf("window %q was never advanced", kind)
				}
			}
		})
	}
}

// TestQuotaCounter_WithoutAStoreIsANoOp pins that a deployment without Redis
// serves: a nil store is the documented "no counters wired" case, not a panic.
func TestQuotaCounter_WithoutAStoreIsANoOp(t *testing.T) {
	cases := []struct {
		name    string
		counter *QuotaCounter
	}{
		{"nil counter", nil},
		{"counter with no store", NewQuotaCounter(nil, quietLogger())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The assertion is that this does not panic: the method is called on
			// a nil receiver in one case, and a store-less counter in the other.
			tc.counter.Record(context.Background(), "ep_one", 100)
		})
	}
}

// TestQuotaCounter_AFailedWriteDoesNotFailTheRequest pins the direction the
// accounting writes follow: the client already has its answer, so a Redis
// failure is logged and the call proceeds.
func TestQuotaCounter_AFailedWriteDoesNotFailTheRequest(t *testing.T) {
	store := &stubCounterStore{err: errors.New("redis is unreachable")}
	counter := NewQuotaCounter(store, quietLogger())

	// No error is returned to assert on: Record has no error return by design,
	// which is what keeps a bookkeeping failure from becoming a client error.
	counter.Record(context.Background(), "ep_one", 100)
	if len(store.adds) != 0 {
		t.Fatalf("writes = %d, want 0 when the store refuses", len(store.adds))
	}
}
