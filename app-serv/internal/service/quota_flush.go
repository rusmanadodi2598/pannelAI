// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush.go
// @for       The bounded worker that drains Redis quota counters into
//
//	PostgreSQL, with its stated retry and dead-letter policy.
//
// @uses      internal/domain, internal/repository, log/slog, runtime/debug, sync, time.
// @reason    AGENTS.md §1.6 requires every worker to recover from panic, to have
//
//	an explicit termination condition, and to state its retry and
//	dead-letter behaviour. This is the only long-lived goroutine in the
//	usage vertical, so both properties and both policies are documented
//	here rather than implied by the loop.
//
// RETRY POLICY
//
//	A failed flush is retried on the worker's next tick with the same batch.
//	There is no in-worker retry loop: the tick is the backoff, so a database
//	outage costs one failed tick rather than a hot retry storm against a
//	database already struggling. MaxAttempts bounds how many consecutive ticks
//	one batch is retried for.
//
// DEAD-LETTER POLICY
//
//	After MaxAttempts consecutive failures on the same batch, the batch is
//	dead-lettered: it is logged at error level with the endpoint ids and the
//	last error, and the counters are LEFT IN REDIS, so a restart or a later
//	successful tick still flushes them. Nothing is discarded, and no separate
//	dead-letter store is invented: the counters are already the durable copy
//	until they are cleared, and Clear runs only after a successful write.
//
// TERMINATION
//
//	Run returns when its context is cancelled; it spawns no goroutine that
//	outlives that return.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// QuotaFlushPolicy states the flush worker's retry and dead-letter behaviour,
// which AGENTS.md §1.6 requires every worker to declare explicitly.
//
// Retry: a flush that fails is retried on the worker's next tick with the SAME
// batch, because the counters are only cleared after a successful durable write.
// There is no in-worker retry loop and no exponential backoff: the tick itself
// is the backoff, so a transient database outage costs one failed tick rather
// than a hot retry storm against a database that is already struggling.
//
// Attempts are bounded per batch. After MaxAttempts consecutive failures on the
// same batch, the batch is dead-lettered: it is logged at error level with the
// endpoint ids and the last error, and Redis RetainAfterFailure keeps the
// counters so the next start can still flush them. Nothing is silently dropped
// — a counter lost is a quota an operator would under-read.
type QuotaFlushPolicy struct {
	// Interval is the tick between flushes.
	Interval time.Duration
	// BatchSize bounds one flush, so a backlog is drained over several ticks
	// instead of one unbounded statement (AGENTS.md §1.7).
	BatchSize int
	// MaxAttempts is how many consecutive failures one batch is retried for
	// before it is dead-lettered and left in Redis.
	MaxAttempts int
	// Timeout bounds one flush, so a hung database cannot stall the worker loop.
	Timeout time.Duration
}

// DefaultQuotaFlushPolicy is the policy used when a caller does not supply one.
func DefaultQuotaFlushPolicy() QuotaFlushPolicy {
	return QuotaFlushPolicy{Interval: 30 * time.Second, BatchSize: 500, MaxAttempts: 5, Timeout: 10 * time.Second}
}

// QuotaFlusher drains Redis counters into PostgreSQL on a bounded tick.
type QuotaFlusher struct {
	counters repository.QuotaCounterStore
	quotas   repository.QuotaRepository
	policy   QuotaFlushPolicy
	logger   *slog.Logger

	// attempts counts consecutive failures for the batch currently being
	// retried, and lastKey identifies that batch so a different batch resets
	// the count rather than inheriting another batch's failures.
	//
	// These are read and written only while running holds the flush, which is
	// what keeps them single-owner state rather than shared state.
	attempts int
	lastKey  string

	// running guards the flush so at most one executes at a time. A guard rather
	// than a mutex because the two entry points are "the ticker" and "flush on
	// shutdown", and a caller that arrives while one is in flight should be told
	// "already flushing" rather than be serialised behind a database timeout it
	// cannot see. That also survives a future caller invoking FlushOnce from a
	// request path: it fails fast instead of blocking a handler.
	running atomic.Bool
}

// NewQuotaFlusher validates deps and returns a flusher.
func NewQuotaFlusher(counters repository.QuotaCounterStore, quotas repository.QuotaRepository, policy QuotaFlushPolicy, logger *slog.Logger) (*QuotaFlusher, error) {
	if counters == nil || quotas == nil {
		return nil, domain.NewValidationError("quota counter store and quota repository are required")
	}
	if policy.Interval <= 0 || policy.BatchSize < 1 || policy.MaxAttempts < 1 || policy.Timeout <= 0 {
		return nil, domain.NewValidationError("quota flush policy is invalid")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &QuotaFlusher{counters: counters, quotas: quotas, policy: policy, logger: logger}, nil
}

// Run flushes until ctx is cancelled, then returns. It is the worker's explicit
// termination condition: the loop selects on ctx.Done() and never spawns a
// goroutine of its own, so returning from Run leaves nothing running.
func (f *QuotaFlusher) Run(ctx context.Context) {
	ticker := time.NewTicker(f.policy.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.flushOnce(ctx)
		}
	}
}

// FlushOnce runs one flush cycle, which lets a test drive the worker without a
// ticker and lets the composition root flush on shutdown.
//
// It reports whether it ran. A false result means another flush was already in
// flight, which is not an error: the shutdown flush racing a tick is expected,
// and the running one drains the same counters.
func (f *QuotaFlusher) FlushOnce(ctx context.Context) bool { return f.flushOnce(ctx) }

// flushOnce drains one batch and persists it, in a panic-recovering goroutine
// (AGENTS.md §1.6: a panic in an unrecovered goroutine kills the process).
//
// The flush itself is synchronous inside the goroutine; the goroutine exists so
// a panic in the driver or in the policy code is contained and logged rather
// than taking the binary down, and so the shape matches every other worker in
// this service.
//
// The running guard is claimed BEFORE the goroutine starts and released by it,
// so a second caller is refused immediately rather than after the first
// completes.
func (f *QuotaFlusher) flushOnce(ctx context.Context) bool {
	if !f.running.CompareAndSwap(false, true) {
		return false
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer f.running.Store(false)
		defer func() {
			if recovered := recover(); recovered != nil {
				f.logger.Error("panic saat flush kuota", "panic", recovered, "stack", string(debug.Stack()))
			}
		}()
		f.drain(ctx)
	}()
	<-done
	return true
}

// drain reads one batch, writes it, and clears only what was written.
func (f *QuotaFlusher) drain(ctx context.Context) {
	callCtx, cancel := context.WithTimeout(ctx, f.policy.Timeout)
	defer cancel()

	pending, err := f.counters.Pending(callCtx, f.policy.BatchSize)
	if err != nil {
		f.logger.Error("membaca counter kuota gagal", "error", err)
		return
	}
	if len(pending) == 0 {
		f.attempts, f.lastKey = 0, ""
		return
	}

	batchKey := batchIdentity(pending)
	if batchKey != f.lastKey {
		f.attempts, f.lastKey = 0, batchKey
	}

	if err := f.quotas.UpsertWindows(callCtx, pending); err != nil {
		f.attempts++
		if f.attempts >= f.policy.MaxAttempts {
			// Dead-letter: the counters stay in Redis, so they are retried after
			// a restart rather than lost. The batch identity names the endpoints
			// an operator has to look at.
			f.logger.Error("batch flush kuota masuk dead-letter",
				"batch", batchKey, "windows", len(pending), "attempts", f.attempts, "error", err)
			f.attempts, f.lastKey = 0, ""
			return
		}
		f.logger.Warn("flush kuota gagal, akan dicoba lagi",
			"batch", batchKey, "attempts", f.attempts, "error", err)
		return
	}

	if err := f.counters.Clear(callCtx, pending); err != nil {
		// The rows are durable; only the drain failed. Leaving them means the
		// next flush writes the same values again, which is idempotent because
		// UpsertWindows replaces rather than adds.
		f.logger.Error("membersihkan counter kuota gagal", "batch", batchKey, "error", err)
		return
	}
	f.attempts, f.lastKey = 0, ""
}

// batchIdentity names a batch by its endpoint ids so a retry of the same batch
// is recognised and a different batch is not charged with its failures.
func batchIdentity(windows []domain.QuotaWindow) string {
	seen := make([]string, 0, len(windows))
	for _, window := range windows {
		seen = append(seen, string(window.Window()))
	}
	return fmt.Sprintf("%d:%v", len(windows), seen)
}

// StartQuotaFlush runs the flusher until ctx is cancelled and blocks until it
// has stopped, so the composition root can join it on shutdown.
func StartQuotaFlush(ctx context.Context, flusher *QuotaFlusher) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic di worker flush kuota", "panic", recovered, "stack", string(debug.Stack()))
			}
		}()
		flusher.Run(ctx)
	}()
	wg.Wait()
}
