// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_lifecycle_test.go
// @for       Tests for the flush worker's lifecycle: the single-flight guard
//
//	across its two goroutines, and its termination on context cancellation.
//
// @uses      context, sync, sync/atomic, testing, time.
// @reason    AGENTS.md §1.6 forbids a fire-and-forget goroutine and requires a
//
//	recovered goroutine; this worker holds retry state across calls, so
//	the guard is asserted under -race rather than argued (draft 005 F9
//	split the original 300-line flusher test, lifecycle cases here).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestQuotaFlusher_FlushOnceRefusesWhileRunning pins the guard's observable
// behaviour: a caller arriving during a flush is told it did not run rather
// than being blocked behind a database timeout it cannot see.
func TestQuotaFlusher_FlushOnceRefusesWhileRunning(t *testing.T) {
	release := make(chan struct{})
	store := &countingStore{hold: release}
	repo := &recordingQuotaRepo{}
	flusher := newFlusherFixture(t, store, repo, testPolicy())

	started := make(chan struct{})
	firstDone := make(chan bool, 1)
	go func() {
		close(started)
		firstDone <- flusher.FlushOnce(context.Background())
	}()

	<-started
	// Wait until the first flush is genuinely inside the store, so this is not a
	// race between the two goroutines' scheduling.
	deadline := time.Now().Add(2 * time.Second)
	for store.inFlight.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the first flush never reached the store")
		}
		time.Sleep(time.Millisecond)
	}

	if ran := flusher.FlushOnce(context.Background()); ran {
		t.Fatal("a concurrent FlushOnce ran, want it refused while one is in flight")
	}

	close(release)
	if ran := <-firstDone; !ran {
		t.Fatal("the first FlushOnce reported it did not run")
	}

	// Once the first has finished, the guard must be released.
	if ran := flusher.FlushOnce(context.Background()); !ran {
		t.Fatal("FlushOnce after the first completed was refused, so the guard leaked")
	}
}

// TestQuotaFlusher_RunStopsOnCancellation proves the worker's termination
// condition: Run returns when its context is cancelled and leaves nothing
// running (AGENTS.md §1.6 forbids a fire-and-forget goroutine).
func TestQuotaFlusher_RunStopsOnCancellation(t *testing.T) {
	cases := []struct {
		name     string
		interval time.Duration
	}{
		{name: "a fast tick", interval: time.Millisecond},
		{name: "a slow tick", interval: time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &countingStore{}
			repo := &recordingQuotaRepo{}
			policy := testPolicy()
			policy.Interval = tc.interval
			flusher := newFlusherFixture(t, store, repo, policy)

			ctx, cancel := context.WithCancel(context.Background())
			returned := make(chan struct{})
			go func() {
				flusher.Run(ctx)
				close(returned)
			}()

			time.Sleep(5 * time.Millisecond)
			cancel()

			select {
			case <-returned:
			case <-time.After(2 * time.Second):
				t.Fatal("Run did not return after cancellation")
			}
		})
	}
}

// TestQuotaFlusher_ConcurrentEntryPointsRunOneFlushAtATime is the race test: Run
// and FlushOnce are reachable from different goroutines, so driving both must
// not admit two flushes into the shared retry state, and the run must be clean
// under -race.
func TestQuotaFlusher_ConcurrentEntryPointsRunOneFlushAtATime(t *testing.T) {
	release := make(chan struct{})
	store := &countingStore{hold: release}
	repo := &recordingQuotaRepo{}
	flusher := newFlusherFixture(t, store, repo, testPolicy())

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// The worker goroutine, as the composition root runs it.
	wg.Add(1)
	go func() {
		defer wg.Done()
		flusher.Run(ctx)
	}()

	// A second goroutine hammering the other entry point while the worker ticks.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			flusher.FlushOnce(context.Background())
		}
	}()

	// Let the contention build, then release every held flush and stop.
	time.Sleep(20 * time.Millisecond)
	close(release)
	cancel()
	wg.Wait()

	if got := store.maxInFlight.Load(); got > 1 {
		t.Fatalf("max concurrent flushes = %d, want at most 1", got)
	}
}
