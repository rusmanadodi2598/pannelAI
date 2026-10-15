// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_sessions_test.go
// @for       Serialization of the calls that share one upstream session: FIFO, cancellation-safe, held until the answer ends, and free of the walk's clock.
// @uses      context, io, net/http, net/http/httptest, sync, sync/atomic, testing, time, internal/provider, internal/registry.
// @reason    The OpenCode free tier throttles per session and the gateway presents one session per (client, endpoint) pair, so parallel requests stacked upstream and were released together (5 parallel requests, ~7s TTFB, observed). One request in flight per session is the fix, and its boundaries are rules: a waiter that is canceled must not block the queue, the slot must outlive the headers because the upstream request is in flight until the body ends, and the wait must not be charged to the failover walk.
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// sessionConnector is a connector that declares a session identity, the way the
// OpenCode connector does for the free tier.
type sessionConnector struct {
	provider.Base
	url     string
	session string
}

func (c *sessionConnector) Endpoint(provider.Request, provider.Credential) (string, error) {
	return c.url, nil
}

func (c *sessionConnector) ApplyAuth(*http.Request, provider.Credential) error { return nil }

func (c *sessionConnector) SessionKey(provider.Request) string { return c.session }

// sessionTransport builds a transport whose connectors point at one server,
// each declaring its own session key.
func sessionTransport(t *testing.T, server *httptest.Server, ids ...string) *Transport {
	t.Helper()
	plugins := make([]provider.Plugin, 0, len(ids))
	for _, id := range ids {
		plugins = append(plugins, &sessionConnector{
			Base:    provider.Base{ID: id, Auth: "no_auth", Format: registry.DefaultFormat},
			url:     server.URL,
			session: "ses_" + id,
		})
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, plugins...)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	return transport
}

// sessionEntry is the registry entry a session test calls.
func sessionEntry(id, url string) registry.Provider {
	return registry.Provider{ID: id, Transport: registry.Transport{BaseURL: url, Format: registry.DefaultFormat}}
}

// doOnce performs one call and closes its answer, so the session slot is
// released the way the engine's own finish releases it.
func doOnce(ctx context.Context, transport *Transport, entry registry.Provider) error {
	upstream, err := transport.Do(ctx, Call{
		Provider: entry, Model: registry.Model{ID: "m"}, Body: []byte(`{"model":"m"}`),
	})
	if err != nil {
		return err
	}
	// reason: the answer is drained so the slot is released; a close error is
	// not actionable in a test.
	_, _ = io.Copy(io.Discard, upstream.Body)
	_ = upstream.Body.Close()
	return nil
}

// TestTransport_SerializesOneSession pins the fix: while one call for a session
// is in flight, a second call for the same session waits locally instead of
// stacking onto the throttled upstream.
func TestTransport_SerializesOneSession(t *testing.T) {
	var mu sync.Mutex
	inflight, peak := 0, 0
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inflight++
		if inflight > peak {
			peak = inflight
		}
		mu.Unlock()
		entered <- struct{}{}
		<-release
		mu.Lock()
		inflight--
		mu.Unlock()
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

	second := make(chan error, 1)
	go func() { second <- doOnce(context.Background(), transport, entry) }()
	select {
	case <-entered:
		t.Fatal("a second call entered while the first held the session")
	case <-time.After(150 * time.Millisecond):
	}

	releaseAll()
	if err := <-first; err != nil {
		t.Fatalf("first Do() error = %v", err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the queued call never ran after the first released the session")
	}
	if err := <-second; err != nil {
		t.Fatalf("second Do() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if peak != 1 {
		t.Fatalf("peak in-flight calls = %d, want one per session", peak)
	}
}

// TestTransport_DifferentSessionsRunConcurrently pins the scope: only calls
// that share a session serialize. Two sessions must both be in flight at once,
// which the test proves by waiting for both arrivals before releasing either.
func TestTransport_DifferentSessionsRunConcurrently(t *testing.T) {
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrived <- struct{}{}
		<-release
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(func() {
		releaseAll()
		server.Close()
	})

	transport := sessionTransport(t, server, "opencode-a", "opencode-b")
	results := make(chan error, 2)
	for _, id := range []string{"opencode-a", "opencode-b"} {
		entry := sessionEntry(id, server.URL)
		go func() { results <- doOnce(context.Background(), transport, entry) }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-arrived:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of two calls from different sessions reached the upstream; sessions must not serialize against each other", i)
		}
	}
	releaseAll()
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	}
}

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
	queue := newSessionQueue()
	releaseFirst, err := queue.acquire(context.Background(), "ses_x")
	if err != nil {
		t.Fatalf("acquiring the session: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queue.acquire(ctx, "ses_x"); err == nil {
		t.Fatal("acquiring with a canceled context = nil error, want the cancellation")
	}

	waiting := make(chan func(), 1)
	go func() {
		release, err := queue.acquire(context.Background(), "ses_x")
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
