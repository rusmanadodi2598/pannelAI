// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/worker_lease.go
// @for       The per-cycle lease that keeps two replicas from running one worker sweep at the same time.
// @uses      context, log/slog, time.
// @reason    The workers call providers and write shared rows, so a second replica doing the same sweep duplicates the spend and can dead-letter an account the first replica just recovered; one acquire around a cycle is cheaper than making every worker idempotent.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     worker
// @stability stable
// @since     2026-10-22
package service

import (
	"context"
	"log/slog"
	"time"
)

// WorkerLease is the compare-and-delete lock a worker cycle holds.
type WorkerLease interface {
	Acquire(ctx context.Context) (bool, error)
	Release(ctx context.Context) error
}

// acquireWorkerLease returns a release function and whether the cycle may run.
// A nil lease keeps single-process behaviour; a denied or failed acquire skips
// the cycle rather than running without the guard.
func acquireWorkerLease(ctx context.Context, lease WorkerLease, logger *slog.Logger, name string) (func(), bool) {
	if lease == nil {
		return nil, true
	}
	acquired, err := lease.Acquire(ctx)
	if err != nil {
		logger.Warn("worker lease could not be acquired", "worker", name, "error", err)
		return nil, false
	}
	if !acquired {
		return nil, false
	}
	return func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := lease.Release(releaseCtx); err != nil {
			logger.Warn("worker lease could not be released", "worker", name, "error", err)
		}
	}, true
}
