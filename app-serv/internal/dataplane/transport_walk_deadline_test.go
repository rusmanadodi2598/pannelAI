// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_walk_deadline_test.go
// @for       The walk clock's other half: one attempt cannot outlive it, an expired walk dials nothing, and a canceled caller surfaces as a timeout.
// @uses      context, errors, net/http, net/http/httptest, sync/atomic, testing, time, internal/registry.
// @reason    The clamp decides whether a retry may start; these cases pin what happens to the attempt already running when the walk ends, so a slow upstream cannot hold the walk open and a hung-up client is not retried. They share the clamp transport fixture but not its arithmetic.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package dataplane

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestTransport_AttemptCannotOutliveTheWalk pins that one credential's attempt
// is bounded by what the walk has left, so a slow upstream cannot hold the walk
// open past its own clock.
func TestTransport_AttemptCannotOutliveTheWalk(t *testing.T) {
	var calls atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	transport, entry := clampTransport(t, server, "slow")
	budget := &RequestBudget{deadline: time.Now().Add(400 * time.Millisecond), total: 30 * time.Second, attempts: 8}

	started := time.Now()
	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m"}`), Budget: budget,
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Do() error = nil, want the walk's timeout")
	}
	if code := AsError(err).Code; code != CodeUpstreamTimeout {
		t.Fatalf("code = %s, want %s", code, CodeUpstreamTimeout)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Do() took %v, want the attempt cut at the walk's clock", elapsed)
	}
	if got := calls.Load(); got > 2 {
		t.Fatalf("upstream calls = %d, want at most the one retry the POST cap allows", got)
	}
}

// TestTransport_ExpiredWalkCallsNothing pins that a walk with no clock left
// spends no credential: the failure is reported before the upstream is dialed.
func TestTransport_ExpiredWalkCallsNothing(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "expired")
	budget := &RequestBudget{deadline: time.Now().Add(-time.Second), total: 30 * time.Second, attempts: 8}

	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m"}`), Budget: budget,
	})
	if err == nil {
		t.Fatal("Do() error = nil, want the expired walk reported")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("upstream calls = %d, want none for an expired walk", got)
	}
}

// TestTransport_ClampSurvivesACanceledCaller pins that a client hang-up while
// the clamp decides is reported as a timeout rather than a retry loop.
func TestTransport_ClampSurvivesACanceledCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "canceled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := transport.Do(ctx, Call{
		Provider: entry, Model: registry.Model{ID: "m"}, Body: []byte(`{"model":"m"}`),
	})
	if err == nil {
		t.Fatal("Do() error = nil, want the canceled caller reported")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want it to wrap the cancellation", err)
	}
}
