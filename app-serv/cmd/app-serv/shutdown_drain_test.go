// Composition root for app-serv: the shutdown path tests.
//
// @file      cmd/app-serv/shutdown_drain_test.go
// @for       The drain the stop path must run, and the order it runs in.
// @uses      context, net, net/http, testing, time
// @reason    The quota flusher's contract says the composition root flushes on shutdown. A stop path that never calls it silently loses the spend since the last tick on every restart, so the capability needs a test that fails when the call disappears.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-04
package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// freeLocalAddr reserves a port, releases it, and returns its address so serve
// can bind it without a fixed port colliding with another process.
func freeLocalAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return addr
}

// waitServing polls until the address answers a TCP dial, so the test does not
// cancel before the listener is actually up.
func waitServing(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("nothing is listening on %s after 2s", addr)
}

func TestServeDrainsCountersAfterTheServerStops(t *testing.T) {
	addr := freeLocalAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	srv := &http.Server{Addr: addr, Handler: http.NewServeMux()}

	var drained bool
	drain := func(context.Context) { drained = true }

	// serve blocks, so it runs on its own goroutine and the readiness poll stays
	// on the test goroutine, which is the only one allowed to fail the test.
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, nil, drain, shutdownTimeout) }()

	waitServing(t, addr)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after its context was cancelled")
	}
	if !drained {
		t.Fatal("the drain never ran, so the counters a restart would carry are still unsettled")
	}
}

func TestServeWithoutADrainStillStopsCleanly(t *testing.T) {
	addr := freeLocalAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	srv := &http.Server{Addr: addr, Handler: http.NewServeMux()}

	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, nil, nil, shutdownTimeout) }()

	waitServing(t, addr)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after its context was cancelled")
	}
}

// TestServeDrainsAfterATimedOutShutdown is the long-stream case the flush exists
// for: the window runs out while a request is still open, and the spend that
// request just made is what a restart would otherwise lose. The drain has to carry
// its own budget too, because the shutdown window is gone by then.
func TestServeDrainsAfterATimedOutShutdown(t *testing.T) {
	addr := freeLocalAddr(t)
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	})
	srv := &http.Server{Addr: addr, Handler: handler}

	drainCtx := make(chan context.Context, 1)
	drain := func(ctx context.Context) { drainCtx <- ctx }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, nil, drain, 50*time.Millisecond) }()
	waitServing(t, addr)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		response, err := http.Get("http://" + addr + "/stream")
		if err == nil {
			_ = response.Body.Close()
		}
	}()
	<-started
	cancel()

	var shutdownErr error
	select {
	case shutdownErr = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after its shutdown window ran out")
	}
	if shutdownErr == nil {
		t.Fatal("serve() reported a clean stop with a request still open when the window ran out")
	}

	select {
	case got := <-drainCtx:
		deadline, ok := got.Deadline()
		if !ok {
			t.Fatal("the drain ran on a context with no deadline of its own")
		}
		if !time.Now().Before(deadline) {
			t.Fatal("the drain ran on the deadline the shutdown had already consumed")
		}
	default:
		t.Fatal("the counters were never flushed after a timed-out shutdown")
	}
	<-clientDone
}

func TestQuotaDrainIsAbsentWithoutAFlusher(t *testing.T) {
	if got := quotaDrain(nil); got != nil {
		t.Fatal("quotaDrain(nil) returned a hook, so a deployment without a flusher would panic at shutdown")
	}
}
