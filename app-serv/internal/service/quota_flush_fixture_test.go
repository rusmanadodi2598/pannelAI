// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush_fixture_test.go
// @for       The in-memory stores the flush worker tests drive: a counting
//
//	store for concurrency, a recording repository for writes.
//
// @uses      context, sync/atomic, testing, time, internal/domain.
// @reason    The flusher's guarantees are cross-goroutine (one flush at a time,
//
//	state kept across attempts), so its tests drive real entry points
//	from real goroutines (AGENTS.md §2.1). The doubles live in their
//	own file so the behaviour tests read as behaviour (draft 005 F9
//	split the 300-line flusher test file while adding the batch
//	identity cases).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-20
package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
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
	settleCalls  atomic.Int64

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

func (s *countingStore) Settle(context.Context, []domain.QuotaWindow) error {
	s.settleCalls.Add(1)
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

func (r *recordingQuotaRepo) PageWindowsByProvider(context.Context, int, int) ([]domain.QuotaWindow, int64, error) {
	return nil, 0, nil
}

func (r *recordingQuotaRepo) GetCap(context.Context, string) (domain.QuotaCap, error) {
	return domain.QuotaCap{}, domain.ErrQuotaCapNotFound
}

func (r *recordingQuotaRepo) SetCap(context.Context, domain.QuotaCap) error { return nil }

// newFlusherFixture binds a flusher to the given stores under the given policy.
func newFlusherFixture(t *testing.T, store repository.QuotaCounterStore, repo *recordingQuotaRepo, policy QuotaFlushPolicy) *QuotaFlusher {
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
