// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_sessions_cancellation_test.go
// @for       Who may leave the session queue, and how long a held slot stays held.
// @uses      context, net/http, net/http/httptest, sync, sync/atomic, testing, time, internal/registry.
// @reason    A client that hangs up while queued must not stay in the way of the calls behind it, and the slot must outlive the response headers because the upstream request is still being served when they arrive. Both are boundaries of the same gate, and they are the two ways a serialized lane can wedge a caller that did nothing wrong.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestTransport_CanceledSessionWaiterLeavesTheQueue pins cancellation safety: a
// client that gives up while queued leaves without the slot, and the calls
// behind it are not blocked.
func TestTransport_CanceledSessionWaiterLeavesTheQueue(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(func() {
		releaseAll()
		server.Close()
	})

	transport := sessionTransport(t, server, "opencode-a")
	entry := sessionEntry("opencode-a", server.URL)

	first := make(chan error, 1)
	go func() { first <- doOnce(context.Background(), transport, entry) }()
	<-entered // the first call holds the session

	ctx, cancel := context.WithCancel(context.Background())
	queued := make(chan error, 1)
	go func() {
		_, err := transport.Do(ctx, Call{Provider: entry, Model: registry.Model{ID: "m"}, Body: []byte(`{"model":"m"}`)})
		queued <- err
	}()
	time.Sleep(100 * time.Millisecond) // let the waiter reach the queue
	cancel()
	select {
	case err := <-queued:
		if err == nil {
			t.Fatal("a canceled waiter returned no error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a canceled waiter stayed blocked, want it to leave the queue")
	}

	third := make(chan error, 1)
	go func() { third <- doOnce(context.Background(), transport, entry) }()
	releaseAll()
	if err := <-first; err != nil {
		t.Fatalf("first Do() error = %v", err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the call behind the canceled waiter never ran")
	}
	if err := <-third; err != nil {
		t.Fatalf("third Do() error = %v", err)
	}
	if extra := len(entered); extra != 0 {
		t.Fatalf("the canceled waiter reached the upstream %d times, want it to leave the queue", extra)
	}
}

// TestTransport_SessionSlotIsHeldUntilTheStreamEnds pins the lifetime: Do
// returns at the headers, so releasing there would let the next request stack
// onto a session that is still being served. The slot follows the body.
func TestTransport_SessionSlotIsHeldUntilTheStreamEnds(t *testing.T) {
	var calls atomic.Int64
	hold := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(hold) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first := calls.Add(1) == 1
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		if first {
			<-hold
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(func() {
		releaseAll()
		server.Close()
	})

	transport := sessionTransport(t, server, "opencode-a")
	entry := sessionEntry("opencode-a", server.URL)

	upstream, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m","stream":true}`), Stream: true,
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want the stream the upstream began", err)
	}

	second := make(chan error, 1)
	go func() { second <- doOnce(context.Background(), transport, entry) }()
	time.Sleep(150 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want the second call queued while the first stream is open", got)
	}

	// Closing the stream ends the upstream request and releases the session.
	_ = upstream.Body.Close()
	select {
	case err := <-second:
		if err != nil {
			t.Fatalf("second Do() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the queued call never ran after the first stream closed")
	}
}
