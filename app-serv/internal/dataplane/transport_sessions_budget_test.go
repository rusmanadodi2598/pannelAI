// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_sessions_budget_test.go
// @for       What a session queue wait costs the walk's clock, and the gate's own arithmetic without a network.
// @uses      context, io, net/http, net/http/httptest, sync, testing, time, internal/registry.
// @reason    Time spent waiting behind another call is not time the upstream took, so the walk must hand it back or a queued request reports a timeout it never earned. The second half tests the same gate with no server at all, because a cancellation bug in the arithmetic would otherwise only surface under load.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestTransport_SessionQueueWaitDoesNotSpendTheWalkBudget pins the documented
// choice: a request waiting for its session did not spend upstream time, so the
// wait is handed back to the walk's clock and the attempt that follows still
// gets its bound. Without the pause this call would report a timeout the moment
// it left the queue, having made no upstream call at all.
func TestTransport_SessionQueueWaitDoesNotSpendTheWalkBudget(t *testing.T) {
	entered := make(chan struct{}, 4)
	hold := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(hold) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-hold
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

	budget := &RequestBudget{deadline: time.Now().Add(250 * time.Millisecond), total: 30 * time.Second, attempts: 8}
	second := make(chan error, 1)
	go func() {
		upstream, err := transport.Do(context.Background(), Call{
			Provider: entry, Model: registry.Model{ID: "m"}, Body: []byte(`{"model":"m"}`), Budget: budget,
		})
		if err != nil {
			second <- err
			return
		}
		// reason: the answer is drained so the slot is released; a close error
		// is not actionable in a test.
		_, _ = io.Copy(io.Discard, upstream.Body)
		_ = upstream.Body.Close()
		second <- nil
	}()

	// The budget's deadline passes while the second call is still queued.
	time.Sleep(400 * time.Millisecond)
	releaseAll()
	if err := <-first; err != nil {
		t.Fatalf("first Do() error = %v", err)
	}
	select {
	case err := <-second:
		if err != nil {
			t.Fatalf("queued Do() error = %v, want the queue wait excluded from the walk's clock", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the queued call never ran")
	}
}

// TestSessionQueue_CanceledWaiterIsRemovedFromTheQueue pins the gate's own
// arithmetic without a network: a canceled waiter never takes the slot, and the
// next release hands it to the waiter behind it.
func TestSessionQueue_CanceledWaiterIsRemovedFromTheQueue(t *testing.T) {
	queue := newSessionQueue(nil)
	releaseFirst, _, err := queue.acquire(context.Background(), "ses_x")
	if err != nil {
		t.Fatalf("acquiring the session: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := queue.acquire(ctx, "ses_x"); err == nil {
		t.Fatal("acquiring with a canceled context = nil error, want the cancellation")
	}

	waiting := make(chan func(), 1)
	go func() {
		release, _, err := queue.acquire(context.Background(), "ses_x")
		if err != nil {
			waiting <- nil
			return
		}
		waiting <- release
	}()
	// Let the waiter reach the queue, so the release below is what hands the
	// slot over rather than an uncontended acquire.
	time.Sleep(50 * time.Millisecond)
	releaseFirst()
	select {
	case release := <-waiting:
		if release == nil {
			t.Fatal("the waiter behind the canceled one failed to acquire")
		}
		release()
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter behind the canceled one never acquired the session")
	}
}
