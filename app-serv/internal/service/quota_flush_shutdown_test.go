// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_shutdown_test.go
// @for       The shutdown flush settling a backlog larger than one batch.
// @uses      context, sync/atomic, testing, time.
// @reason    The ticker flushes one batch per tick on purpose, and shutdown is the one moment with no next tick to wait for: a counter left unsettled there is spend the panel reads as missing until the process starts again.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-08
package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// errFlushTest stands in for the driver refusing a write, which is the failure the shutdown
// loop is supposed to stop on rather than retry for its whole window.
var errFlushTest = errors.New("the write was refused")

// TestQuotaFlusher_FlushUntilQuietEmptiesTheBacklog is the case one batch cannot
// cover: three ceiling-sized batches queued, and a drain that takes one and returns
// leaves two unsettled.
func TestQuotaFlusher_FlushUntilQuietEmptiesTheBacklog(t *testing.T) {
	policy := testPolicy()
	store := &countingStore{fullBatches: 3}
	repo := &recordingQuotaRepo{}
	flusher := newFlusherFixture(t, store, repo, policy)

	flusher.FlushUntilQuiet(context.Background())

	// Four reads, not three: the loop has to see a short batch before it knows the
	// keyspace ran out, because Pending reads live state and offers no count of its own.
	if got := store.pendingCalls.Load(); got != 4 {
		t.Fatalf("Pending calls = %d, want 4: three full batches then one short answer", got)
	}
	if got := repo.upserts.Load(); got != 4 {
		t.Fatalf("upserts = %d, want the three backlog batches and the final short one", got)
	}
	if got := store.settleCalls.Load(); got != 4 {
		t.Fatalf("settles = %d, want every written batch settled", got)
	}
}

// TestQuotaFlusher_FlushOnceTakesOneBatchOnly pins the half that must not change: the
// ticker's contract is one bounded batch per cycle (quota_flush_policy.go names it),
// so a backlog has to survive as multiple ticks rather than become one unbounded
// statement (AGENTS.md §1.7).
func TestQuotaFlusher_FlushOnceTakesOneBatchOnly(t *testing.T) {
	store := &countingStore{fullBatches: 3}
	repo := &recordingQuotaRepo{}
	flusher := newFlusherFixture(t, store, repo, testPolicy())

	if !flusher.FlushOnce(context.Background()) {
		t.Fatal("FlushOnce() reported it did not run")
	}
	if got := store.pendingCalls.Load(); got != 1 {
		t.Fatalf("Pending calls = %d, want 1: the ticked path must not drain the backlog itself", got)
	}
}

// TestQuotaFlusher_FlushUntilQuietStopsOnAFailedBatch keeps the loop from spending
// the whole shutdown window on a batch that cannot move: the counters stay in Redis
// and the next boot retries them.
func TestQuotaFlusher_FlushUntilQuietStopsOnAFailedBatch(t *testing.T) {
	store := &countingStore{fullBatches: 3}
	repo := &recordingQuotaRepo{upsertErr: errFlushTest}
	flusher := newFlusherFixture(t, store, repo, testPolicy())

	done := make(chan struct{})
	go func() {
		flusher.FlushUntilQuiet(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("FlushUntilQuiet kept looping on a batch whose write failed")
	}
	if got := store.pendingCalls.Load(); got != 1 {
		t.Fatalf("Pending calls = %d, want it to stop at the first failure", got)
	}
}

// TestQuotaFlusher_FlushUntilQuietHonoursCancellation is the bounded-window case: the
// shutdown deadline must be able to end the loop even with a backlog still queued.
func TestQuotaFlusher_FlushUntilQuietHonoursCancellation(t *testing.T) {
	store := &countingStore{fullBatches: 1000}
	repo := &recordingQuotaRepo{}
	flusher := newFlusherFixture(t, store, repo, testPolicy())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := time.Now()
	flusher.FlushUntilQuiet(ctx)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("FlushUntilQuiet ran %v past a 50ms deadline, so the window cannot end it", elapsed)
	}
}
