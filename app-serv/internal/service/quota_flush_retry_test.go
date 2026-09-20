// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_retry_test.go
// @for       Tests for the flush worker's retry and dead-letter policy, and the
//
//	batch identity that keeps one batch's failures off another's.
//
// @uses      context, sync/atomic, testing, time, internal/domain.
// @reason    The stated policy (quota_flush_policy.go) is a promise about what
//
//	happens to counters when a write keeps failing: they are retried a
//	bounded number of times and then left in Redis, never dropped. Draft
//	005 F4 found the identity that decides "the same batch" keyed to the
//	wrong field, so one batch could be dead-lettered for another's
//	failures; these cases lock the per-endpoint behaviour.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// endpointStore returns one batch of windows for the endpoints it is given, so
// a test controls which endpoint a failing flush is attributed to.
type endpointStore struct {
	endpoints  []string
	clearCalls atomic.Int64
}

func (s *endpointStore) Add(context.Context, string, domain.QuotaWindowKind, int64, time.Time) error {
	return nil
}

func (s *endpointStore) Pending(context.Context, int) ([]domain.QuotaWindow, error) {
	windows := make([]domain.QuotaWindow, 0, len(s.endpoints))
	for _, endpointID := range s.endpoints {
		window, err := domain.NewQuotaWindow(endpointID, "openai", domain.QuotaWindowDaily, nil, nil, time.Now())
		if err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	return windows, nil
}

func (s *endpointStore) Clear(context.Context, []domain.QuotaWindow) error {
	s.clearCalls.Add(1)
	return nil
}

// TestQuotaFlusher_RetriesThenDeadLetters covers the stated retry policy: a
// failing batch is retried up to MaxAttempts and then abandoned WITHOUT clearing
// the counters, so nothing is lost.
func TestQuotaFlusher_RetriesThenDeadLetters(t *testing.T) {
	cases := []struct {
		name        string
		maxAttempts int
		failFlushes int
		wantUpserts int64
		wantClears  int64
	}{
		{name: "a success on the first attempt writes once", maxAttempts: 3, failFlushes: 0, wantUpserts: 1, wantClears: 1},
		{name: "failures below the cap never clear", maxAttempts: 3, failFlushes: 2, wantUpserts: 0, wantClears: 0},
		{name: "hitting the cap dead-letters without clearing", maxAttempts: 2, failFlushes: 2, wantUpserts: 0, wantClears: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &countingStore{}
			repo := &recordingQuotaRepo{upsertErr: errUpsertFailed}
			flusher := newFlusherFixture(t, store, repo, QuotaFlushPolicy{
				Interval: time.Hour, BatchSize: 10, MaxAttempts: tc.maxAttempts, Timeout: time.Second,
			})

			for i := 0; i < tc.failFlushes; i++ {
				flusher.FlushOnce(context.Background())
			}
			if tc.failFlushes == 0 {
				// The successful case needs a working repository.
				repo.upsertErr = nil
				flusher.FlushOnce(context.Background())
			}

			if got := repo.upserts.Load(); got != tc.wantUpserts {
				t.Fatalf("upserts = %d, want %d", got, tc.wantUpserts)
			}
			if got := store.clearCalls.Load(); got != tc.wantClears {
				t.Fatalf("clears = %d, want %d", got, tc.wantClears)
			}
		})
	}
}

// TestQuotaFlusher_AttemptsArePerEndpoint is the F4 definition of done: two
// failing batches with different endpoint ids and the same window kind are two
// batches, so neither can be dead-lettered for the other's failures. With two
// endpoints alternating and MaxAttempts 2, a kind-keyed identity would count
// them as one batch and dead-letter after the second failure; the per-endpoint
// identity keeps each batch below the cap, so Redis is never left holding a
// batch the worker has given up on.
func TestQuotaFlusher_AttemptsArePerEndpoint(t *testing.T) {
	cases := []struct {
		name      string
		endpoints []string
	}{
		{name: "one window kind across two endpoints alternates batches", endpoints: []string{"ep_alpha", "ep_beta"}},
		{name: "three endpoints alternate without sharing an attempt count", endpoints: []string{"ep_alpha", "ep_beta", "ep_gamma"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &endpointStore{}
			repo := &recordingQuotaRepo{upsertErr: errUpsertFailed}
			flusher := newFlusherFixture(t, store, repo, QuotaFlushPolicy{
				Interval: time.Hour, BatchSize: 10, MaxAttempts: 2, Timeout: time.Second,
			})

			// Each flush sees a different endpoint, so each arrival is a new batch
			// and the attempt count resets. With a kind-keyed identity every
			// arrival would look like the same batch and the second would
			// dead-letter.
			for _, endpointID := range tc.endpoints {
				store.endpoints = []string{endpointID}
				flusher.FlushOnce(context.Background())
				if flusher.lastKey == "" {
					t.Fatalf("batch %s was dead-lettered: its identity ignored the endpoint id", endpointID)
				}
			}
		})
	}
}

// TestBatchIdentity covers the identity itself as a table: the same set of
// windows names one batch whatever order it arrives in, and a different
// endpoint or window kind names a different one.
func TestBatchIdentity(t *testing.T) {
	window := func(t *testing.T, endpointID string, kind domain.QuotaWindowKind) domain.QuotaWindow {
		t.Helper()
		built, err := domain.NewQuotaWindow(endpointID, "openai", kind, nil, nil, time.Now())
		if err != nil {
			t.Fatalf("NewQuotaWindow() error = %v", err)
		}
		return built
	}

	alphaDaily := window(t, "ep_alpha", domain.QuotaWindowDaily)
	betaDaily := window(t, "ep_beta", domain.QuotaWindowDaily)
	alphaWeekly := window(t, "ep_alpha", domain.QuotaWindowWeekly)

	cases := []struct {
		name     string
		left     []domain.QuotaWindow
		right    []domain.QuotaWindow
		wantSame bool
	}{
		{
			name: "the same windows in a different order are one batch",
			left: []domain.QuotaWindow{alphaDaily, betaDaily}, right: []domain.QuotaWindow{betaDaily, alphaDaily},
			wantSame: true,
		},
		{
			name: "a single window equals itself",
			left: []domain.QuotaWindow{alphaDaily}, right: []domain.QuotaWindow{alphaDaily},
			wantSame: true,
		},
		{
			name: "the same kind on another endpoint is another batch",
			left: []domain.QuotaWindow{alphaDaily}, right: []domain.QuotaWindow{betaDaily},
			wantSame: false,
		},
		{
			name: "another window kind on the same endpoint is another batch",
			left: []domain.QuotaWindow{alphaDaily}, right: []domain.QuotaWindow{alphaWeekly},
			wantSame: false,
		},
		{
			name: "an extra window changes the batch",
			left: []domain.QuotaWindow{alphaDaily}, right: []domain.QuotaWindow{alphaDaily, betaDaily},
			wantSame: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			same := batchIdentity(tc.left) == batchIdentity(tc.right)
			if same != tc.wantSame {
				t.Fatalf("batchIdentity equal = %v, want %v (%q vs %q)",
					same, tc.wantSame, batchIdentity(tc.left), batchIdentity(tc.right))
			}
		})
	}
}

// errUpsertFailed stands in for a database write failure.
var errUpsertFailed = errorString("upsert failed")

// errorString is a minimal error so the test needs no dependency.
type errorString string

func (e errorString) Error() string { return string(e) }
