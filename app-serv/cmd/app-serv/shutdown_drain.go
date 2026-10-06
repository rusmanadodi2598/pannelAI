// Composition root for app-serv: the shutdown path.
//
// @file      cmd/app-serv/shutdown_drain.go
// @for       The graceful stop of the HTTP server and the counter drain that must follow it.
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

// serve runs the HTTP server until it receives a termination signal, then shuts
// it down within window. Every goroutine here has an explicit termination
// condition (AGENTS.md §1.6).
//
// drain, when present, runs after the server goroutine returns and whether or not
// the stop succeeded. Settling counters is only correct once nothing can still add
// to them, but a window that ran out with a stream still open is the case the flush
// exists for, and skipping it would discard the spend that request just made. It
// gets a window of its own because the shutdown spent the first one.
func serve(ctx context.Context, srv *http.Server, drain func(context.Context), window time.Duration) error {
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
// A false result from FlushOnce is not a failure: it means a tick is already in
// flight, and that tick drains the same counters this process was about to.
func quotaDrain(flusher *service.QuotaFlusher) func(context.Context) {
	if flusher == nil {
		return nil
	}
	return func(ctx context.Context) {
		if flusher.FlushOnce(ctx) {
			slog.Info("quota counters drained at shutdown")
			return
		}
		slog.Info("quota flush already in flight at shutdown")
	}
}
