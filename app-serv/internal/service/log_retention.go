// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/log_retention.go
// @for       The bounded worker that removes request logs outside the configured
//
//	retention window.
//
// @uses      context, errors, log/slog, runtime/debug, sync/atomic, time.
// @reason    SPEC-API-001 §7.13 makes request-log retention a worker concern, and
//
//	AGENTS.md §1.6 requires every worker to state its retry policy,
//	recover panics, and terminate explicitly. Keeping the worker over
//	the LogService purge port means the scheduled path and the manual
//	purge route share the same settings lookup and cutoff semantics.
//
// RETRY POLICY
//
//	A failed purge is retried on the next tick. MaxAttempts bounds consecutive
//	failures; reaching it emits an error and resets the counter, while the next
//	tick starts a fresh cycle. The PostgreSQL DELETE is set-based and atomic, so
//	a failed call leaves the rows for the next attempt.
//
// TERMINATION
//
//	Run returns when ctx is cancelled. Its ticker is stopped explicitly and the
//	purge call has its own timeout, so a worker cannot wait forever on storage.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// LogPurger is the service port used by the retention worker. LogService is the
// production implementation; the narrow interface keeps the worker independent
// of repositories and lets a test drive one purge cycle without a database.
type LogPurger interface {
	Purge(ctx context.Context) (int64, error)
}

// LogRetentionPolicy states when a purge runs and how a failed cycle is retried.
type LogRetentionPolicy struct {
	// Interval is the time between purge cycles.
	Interval time.Duration
	// MaxAttempts bounds consecutive failed cycles before the worker resets its
	// retry counter and reports a dead-letter-style error.
	MaxAttempts int
	// Timeout bounds one call into the purge service.
	Timeout time.Duration
}

// DefaultLogRetentionPolicy is the process policy used by the composition root.
func DefaultLogRetentionPolicy() LogRetentionPolicy {
	return LogRetentionPolicy{Interval: time.Hour, MaxAttempts: 3, Timeout: 10 * time.Second}
}

// LogRetentionWorker periodically invokes the canonical log purge operation.
type LogRetentionWorker struct {
	purger   LogPurger
	policy   LogRetentionPolicy
	logger   *slog.Logger
	attempts int
	running  atomic.Bool
}

// NewLogRetentionWorker validates dependencies and returns a ready worker.
func NewLogRetentionWorker(purger LogPurger, policy LogRetentionPolicy, logger *slog.Logger) (*LogRetentionWorker, error) {
	if purger == nil {
		return nil, domain.NewValidationError("log purger is required")
	}
	if policy.Interval <= 0 || policy.MaxAttempts < 1 || policy.Timeout <= 0 {
		return nil, domain.NewValidationError("log retention policy is invalid")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LogRetentionWorker{purger: purger, policy: policy, logger: logger}, nil
}

// Run purges until ctx is cancelled. The ticker is the explicit schedule and is
// stopped on every return, while RunOnce's timeout bounds each storage call.
func (w *LogRetentionWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.policy.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce executes one purge cycle. It returns false when another cycle is
// already running, which keeps retry state single-owner if shutdown or a test
// invokes the operation while the ticker is active.
func (w *LogRetentionWorker) RunOnce(ctx context.Context) (deleted int64, ran bool) {
	if !w.running.CompareAndSwap(false, true) {
		return 0, false
	}
	defer w.running.Store(false)
	defer func() {
		if recovered := recover(); recovered != nil {
			w.logger.Error("panic in log retention worker", "panic", recovered, "stack", string(debug.Stack()))
			w.recordFailure(errors.New("log retention purge panicked"))
			deleted = 0
			ran = true
		}
	}()

	callCtx, cancel := context.WithTimeout(ctx, w.policy.Timeout)
	defer cancel()
	deleted, err := w.purger.Purge(callCtx)
	if err != nil {
		w.recordFailure(err)
		return 0, true
	}
	w.attempts = 0
	w.logger.Info("request log retention purge completed", "deleted", deleted)
	return deleted, true
}

// recordFailure applies the fixed-tick retry policy. There is no durable
// dead-letter table for logs; a failed DELETE leaves the rows untouched, so the
// next process start can retry the same retention window.
func (w *LogRetentionWorker) recordFailure(err error) {
	w.attempts++
	if w.attempts >= w.policy.MaxAttempts {
		w.logger.Error("request log retention purge reached retry limit",
			"attempts", w.attempts, "error", err)
		w.attempts = 0
		return
	}
	w.logger.Warn("request log retention purge failed; will retry",
		"attempts", w.attempts, "error", err)
}
