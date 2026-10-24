// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/usage_retention.go
// @for       The worker that removes usage records outside the configured retention window.
// @uses      context, log/slog, runtime/debug, sync/atomic, time.
// @reason    usage_records is written on every served call and has no expiry of its own, so an explicit worker is the only thing that keeps the billing history bounded; it runs daily and states its retry and panic rules like every other worker.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// UsageRetentionPolicy states when the usage purge runs and how a failed cycle is
// retried.
type UsageRetentionPolicy struct {
	Interval    time.Duration
	Retention   time.Duration
	MaxAttempts int
	Timeout     time.Duration
}

// DefaultUsageRetentionPolicy builds the process policy from the configured day
// count. A non-positive value falls back to 90 days rather than deleting nothing.
func DefaultUsageRetentionPolicy(days int) UsageRetentionPolicy {
	if days < 1 {
		days = 90
	}
	return UsageRetentionPolicy{
		Interval: 24 * time.Hour, Retention: time.Duration(days) * 24 * time.Hour,
		MaxAttempts: 3, Timeout: 30 * time.Second,
	}
}

// UsagePurger removes usage rows older than a cutoff.
type UsagePurger interface {
	DeleteUsageOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// UsageRetentionWorker periodically removes usage records outside the window.
type UsageRetentionWorker struct {
	purger   UsagePurger
	policy   UsageRetentionPolicy
	logger   *slog.Logger
	attempts int
	running  atomic.Bool
	now      func() time.Time
}

// NewUsageRetentionWorker validates dependencies and returns a ready worker.
func NewUsageRetentionWorker(purger UsagePurger, policy UsageRetentionPolicy, logger *slog.Logger) (*UsageRetentionWorker, error) {
	if purger == nil {
		return nil, domain.NewValidationError("usage purger is required")
	}
	if policy.Interval <= 0 || policy.Retention <= 0 || policy.MaxAttempts < 1 || policy.Timeout <= 0 {
		return nil, domain.NewValidationError("usage retention policy is invalid")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &UsageRetentionWorker{purger: purger, policy: policy, logger: logger, now: time.Now}, nil
}

// Run purges until ctx is cancelled, on the policy's interval.
func (w *UsageRetentionWorker) Run(ctx context.Context) {
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

// RunOnce executes one purge cycle, guarded so a shutdown or a test cannot run
// two cycles at once.
func (w *UsageRetentionWorker) RunOnce(ctx context.Context) (deleted int64, ran bool) {
	if !w.running.CompareAndSwap(false, true) {
		return 0, false
	}
	defer w.running.Store(false)
	defer func() {
		if recovered := recover(); recovered != nil {
			w.logger.Error("panic in usage retention worker", "panic", recovered, "stack", string(debug.Stack()))
			w.recordFailure(domain.NewInternalError("usage retention purge panicked"))
			deleted = 0
			ran = true
		}
	}()

	callCtx, cancel := context.WithTimeout(ctx, w.policy.Timeout)
	defer cancel()
	cutoff := w.now().UTC().Add(-w.policy.Retention)
	deleted, err := w.purger.DeleteUsageOlderThan(callCtx, cutoff)
	if err != nil {
		w.recordFailure(err)
		return 0, true
	}
	w.attempts = 0
	w.logger.Info("usage retention purge completed", "deleted", deleted, "cutoff", cutoff)
	return deleted, true
}

func (w *UsageRetentionWorker) recordFailure(err error) {
	w.attempts++
	if w.attempts >= w.policy.MaxAttempts {
		w.logger.Error("usage retention purge reached retry limit", "attempts", w.attempts, "error", err)
		w.attempts = 0
		return
	}
	w.logger.Warn("usage retention purge failed; will retry", "attempts", w.attempts, "error", err)
}
