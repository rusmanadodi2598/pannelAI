// Composition root for app-serv: the shutdown path.
//
// @file      cmd/app-serv/shutdown_drain.go
// @for       The graceful stop of the HTTP server, the background workers that must end after it, and the counter drain that must follow both.
// @uses      context, errors, log/slog, net/http, sync, time, internal/service (the quota flusher).
// @reason    The quota flusher settles Redis counters on a tick, and its own contract says the composition root flushes on shutdown; without that call every restart discarded the spend since the last tick and the panel read a total lower than the keys actually paid.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-04
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// serve runs the HTTP server until it receives a termination signal, then shuts it down
// within window. Every goroutine here has an explicit termination condition (AGENTS.md §1.6).
//
// stopWorkers runs after the server has drained: a worker stopped by the signal drains
// its own queue while handlers are still feeding it.
//
// drain runs last, whether or not the stop succeeded, with a window of its own: the
// spend of a request cut off by the timeout is exactly what a restart would lose.
func serve(ctx context.Context, srv *http.Server, stopWorkers func(time.Duration),
	drain func(context.Context), window time.Duration) error {
	serverErr := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("server goroutine panic recovered", "panic", r)
			}
		}()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	slog.Info("app-serv is listening", "addr", srv.Addr)

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-serverErr:
		return err
	}

	// The parent ctx is already cancelled here, so deriving this timeout from it
	// would abort the drain instantly: context.Background() is the correct root
	// for a bounded shutdown window.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), window)
	defer cancel()
	//nolint:contextcheck // reason: ctx is already cancelled, so it cannot root the drain timeout.
	shutdownErr := srv.Shutdown(shutdownCtx)
	wg.Wait()
	if stopWorkers != nil {
		stopWorkers(window)
	}
	if drain != nil {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), window)
		defer drainCancel()
		//nolint:contextcheck // reason: the drain must run after cancellation, in a window the shutdown did not spend.
		drain(drainCtx)
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	slog.Info("app-serv stopped cleanly")
	return nil
}

// quotaDrain returns the shutdown flush the quota worker documents, or nil for a
// deployment built without one.
//
// A false result is not a failure: it means a tick is already in flight, and that tick
// drains the same counters this process was about to.
func quotaDrain(flusher *service.QuotaFlusher) func(context.Context) {
	if flusher == nil {
		return nil
	}
	return func(ctx context.Context) {
		// Until quiet, not one batch: a backlog larger than BatchSize would otherwise
		// stay in Redis, and shutdown is the one moment with no next tick to settle it.
		if flusher.FlushUntilQuiet(ctx) {
			slog.Info("quota counters drained at shutdown")
			return
		}
		slog.Info("quota flush already in flight at shutdown")
	}
}
