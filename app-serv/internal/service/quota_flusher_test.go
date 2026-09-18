// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flusher_test.go
// @for       Concurrency and policy tests for the quota flush worker.
// @uses      testing, context, sync, sync/atomic, time, internal/domain,
//
//	internal/repository.
//
// @reason    The flusher has two documented entry points driven from different
//
//	goroutines: the ticker in Run and the shutdown flush. AGENTS.md §2.1
//	treats a race-detector failure as blocking, and this worker holds
//	retry state across calls, so the guarantee "at most one flush runs
//	at a time" is asserted under -race rather than argued in a comment.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// countingStore is a QuotaCounterStore that records how many flushes are in
// flight at once, so the test can prove the guard admits one.
type countingStore struct {
	// inFlight is how many Pending calls are being served right now, and
	// maxInFlight is the high-water mark. A value above 1 means the guard let a
	// second flush into the retry state concurrently.
	inFlight    atomic.Int64
	maxInFlight atomic.Int64

	pendingCalls atomic.Int64
	clearCalls   atomic.Int64

	// hold lets a test keep one flush inside the store while another arrives.
	hold    chan struct{}
	pendErr error
}

func (s *countingStore) Add(context.Context, string, domain.QuotaWindowKind, int64, time.Time) error {
	return nil
}

func (s *countingStore) Pending(ctx context.Context, limit int) ([]domain.QuotaWindow, error) {
	s.pendingCalls.Add(1)
	current := s.inFlight.Add(1)
	for {
		high := s.maxInFlight.Load()
		if current <= high || s.maxInFlight.CompareAndSwap(high, current) {
			break
		}
	}
	defer s.inFlight.Add(-1)

	if s.hold != nil {
		select {
		case <-s.hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.pendErr != nil {
		return nil, s.pendErr
	}
	window, err := domain.NewQuotaWindow("ep_1", "openai", domain.QuotaWindowDaily, nil, nil, time.Now())
	if err != nil {
		return nil, err
	}
	window.Add(10, time.Now())
	return []domain.QuotaWindow{window}, nil
}

func (s *countingStore) Clear(context.Context, []domain.QuotaWindow) error {
	s.clearCalls.Add(1)
	return nil
}

// recordingQuotaRepo is a QuotaRepository that records writes and can fail.
type recordingQuotaRepo struct {
	upserts   atomic.Int64
	upsertErr error
}

func (r *recordingQuotaRepo) UpsertWindows(context.Context, []domain.QuotaWindow) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.upserts.Add(1)
	return nil
}

// The flusher only writes windows; the two reads and the cap write are part of
// the interface it is bound to, so a fixture that would compile without them
// would silently not represent a real QuotaRepository.
func (r *recordingQuotaRepo) ListWindows(context.Context, string) ([]domain.QuotaWindow, error) {
	return nil, nil
}

func (r *recordingQuotaRepo) GetCap(context.Context, string) (domain.QuotaCap, error) {
	return domain.QuotaCap{}, domain.ErrQuotaCapNotFound
}

func (r *recordingQuotaRepo) SetCap(context.Context, domain.QuotaCap) error { return nil }

func newFlusherFixture(t *testing.T, store *countingStore, repo *recordingQuotaRepo, policy QuotaFlushPolicy) *QuotaFlusher {
	t.Helper()
	flusher, err := NewQuotaFlusher(store, repo, policy, nil)
	if err != nil {
		t.Fatalf("NewQuotaFlusher() error = %v", err)
	}
	return flusher
}

func testPolicy() QuotaFlushPolicy {
	return QuotaFlushPolicy{
		Interval:    time.Millisecond,
		BatchSize:   10,
		MaxAttempts: 3,
		Timeout:     2 * time.Second,
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

// TestQuotaFlusher_FlushOnceRefusesWhileRunning pins the guard's observable
// behaviour: a caller arriving during a flush is told it did not run rather than
// being blocked behind a database timeout it cannot see.
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

// errUpsertFailed stands in for a database write failure.
var errUpsertFailed = errorString("upsert failed")

// errorString is a minimal error so the test needs no dependency.
type errorString string

func (e errorString) Error() string { return string(e) }
