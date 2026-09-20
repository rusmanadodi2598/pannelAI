// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_flush.go
// @for       The quota flush worker's lifecycle: one bounded goroutine, a
//
//	single-flight guard, and the composition-root entry point.
//
// @uses      internal/domain, internal/repository, log/slog, runtime/debug,
//
//	sync, sync/atomic, time.
//
// @reason    AGENTS.md §1.6 requires every worker to recover from panic and to
//
//	have an explicit termination condition, which is lifecycle, not
//	batch mechanics: this file states who runs, when, and how it stops,
//	while the batch itself lives in quota_flush_drain.go and the stated
//	retry and dead-letter behaviour in quota_flush_policy.go
//	(draft 005 F3).
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
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

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
