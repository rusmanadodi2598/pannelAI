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

// The queue's other rules live in their own files, one concern each: what a
// canceled waiter and an open stream do to the slot
// (transport_sessions_cancellation_test.go), how the wait is charged to the walk
// (transport_sessions_budget_test.go), and what a stall reports to the log
// (transport_sessions_stall_test.go).
