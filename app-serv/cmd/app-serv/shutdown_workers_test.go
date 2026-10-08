// Composition root for app-serv: the worker half of the stop path.
//
// @file      cmd/app-serv/shutdown_workers_test.go
// @for       The order the stop path ends the background workers in, and the join that waits for them.
// @uses      context, net/http, sync/atomic, testing, time.
// @reason    A worker stopped when the signal arrives drains its queue while handlers are still running, so what those requests record lands where no consumer is left, and a shutdown that never joins its goroutines returns while they are still writing. Both are only visible as an order between two events, which is what these measure.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-08
package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// TestServeStopsWorkersAfterTheInFlightRequestFinished is the ordering the fix is
// about: the signal arrives while a handler is still running, that handler must
// finish before the workers are stopped, and the stop must still happen at all.
func TestServeStopsWorkersAfterTheInFlightRequestFinished(t *testing.T) {
	addr := freeLocalAddr(t)

	handlerFinished := make(chan time.Time, 1)
	server := &http.Server{Addr: addr, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		handlerFinished <- time.Now()
		_, _ = w.Write([]byte("done"))
	})}

	stoppedAt := make(chan time.Time, 1)
	stopWorkers := func(time.Duration) { stoppedAt <- time.Now() }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, stopWorkers, nil, shutdownTimeout) }()
	waitServing(t, addr)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		response, err := http.Get("http://" + addr + "/slow")
		if err == nil {
			_ = response.Body.Close()
		}
	}()

	// Cancel while the handler is genuinely mid-flight, which is the case that used
	// to strand the record it was about to produce.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after its context was cancelled")
	}

	var handler, stopped time.Time
	select {
	case handler = <-handlerFinished:
	case <-time.After(2 * time.Second):
		t.Fatal("the in-flight handler never completed")
	}
	select {
	case stopped = <-stoppedAt:
	case <-time.After(2 * time.Second):
		t.Fatal("the workers were never stopped, so nothing joins them at shutdown")
	}
	if stopped.Before(handler) {
		t.Fatalf("workers stopped at %v, before the handler finished at %v: the ordering is the fix",
			stopped.Format(time.StampMilli), handler.Format(time.StampMilli))
	}
	<-clientDone
}

// newTestWorkerGroup builds a group with no dependencies, so the join can be measured
// on its own.
func newTestWorkerGroup() *workerGroup {
	group := &workerGroup{}
	group.ctx, group.cancel = context.WithCancel(context.Background())
	return group
}

// TestWorkerGroupJoinsEveryWorker is the half that was missing entirely: the workers
// were spawned with a bare `go`, so serve returned while they were still running.
func TestWorkerGroupJoinsEveryWorker(t *testing.T) {
	group := newTestWorkerGroup()
	var returned atomic.Int64
	for range 3 {
		group.start("counter", func() {
			<-group.ctx.Done()
			returned.Add(1)
		})
	}

	group.stopAndJoin(2 * time.Second)

	if got := returned.Load(); got != 3 {
		t.Fatalf("workers still running after stopAndJoin = %d, want 0: the join returned early", 3-got)
	}
}

// TestWorkerGroupStopAndJoinIsBounded keeps one worker that ignores its context from
// holding the process open past what an orchestrator's kill timeout allows.
func TestWorkerGroupStopAndJoinIsBounded(t *testing.T) {
	group := newTestWorkerGroup()
	group.start("stuck", func() { <-make(chan struct{}) })

	window := 80 * time.Millisecond
	started := time.Now()
	group.stopAndJoin(window)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("stopAndJoin waited %v past a %v window, so a stuck worker delays the whole shutdown",
			elapsed, window)
	}
}

// TestWorkerContextSurvivesTheSignal is the other half of the ordering: the workers
// must not be cancelled by the signal's context, or the stop path never gets to choose
// the moment.
func TestWorkerContextSurvivesTheSignal(t *testing.T) {
	signal, cancelSignal := context.WithCancel(context.Background())
	workerCtx, cancelWorkers := workerContext(signal)

	cancelSignal()
	select {
	case <-workerCtx.Done():
		t.Fatal("the worker context died with the signal, so the stop path cannot order it")
	default:
	}

	cancelWorkers()
	select {
	case <-workerCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("stopAndJoin's cancel never reached the workers")
	}
}
