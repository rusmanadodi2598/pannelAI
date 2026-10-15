// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_clamp_test.go
// @for       The fail-fast clamp: a wait or an attempt that cannot fit in what is left of the failover walk is not started.
// @uses      context, net/http, net/http/httptest, sync/atomic, testing, time, internal/provider, internal/registry.
// @reason    The walk budget was checked only between credentials, so one credential could sleep an honored Retry-After of minutes and spend the whole walk, and the walk then reported UPSTREAM_TIMEOUT instead of the RATE_LIMITED the upstream had already sent. The clamp arithmetic is a pure function and the transport-level consequence is pinned against a real stand-in upstream.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestClampWait pins the arithmetic: a wait that fits is slept whole, and a
// wait that would spend the rest of the walk (or all of it) is not started,
// because the retry it was waiting for would begin after the walk expired.
func TestClampWait(t *testing.T) {
	cases := []struct {
		name      string
		wait      time.Duration
		remaining time.Duration
		want      time.Duration
		wantOK    bool
	}{
		{name: "a wait inside the walk is slept whole", wait: 2 * time.Second, remaining: 30 * time.Second,
			want: 2 * time.Second, wantOK: true},
		{name: "a wait that is the whole walk is not started", wait: 30 * time.Second, remaining: 30 * time.Second,
			wantOK: false},
		{name: "a wait beyond the walk is not started", wait: 60 * time.Second, remaining: 5 * time.Second,
			wantOK: false},
		{name: "an expired walk starts nothing", wait: time.Second, remaining: 0, wantOK: false},
		{name: "a negative remainder is an expired walk", wait: time.Second, remaining: -time.Millisecond,
			wantOK: false},
		{name: "no wait is always fine", wait: 0, remaining: 0, wantOK: true},
		{name: "a negative wait is no wait", wait: -time.Second, remaining: 0, wantOK: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := clampWait(tc.wait, tc.remaining)
			if ok != tc.wantOK {
				t.Fatalf("clampWait(%v, %v) ok = %v, want %v", tc.wait, tc.remaining, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("clampWait(%v, %v) = %v, want %v", tc.wait, tc.remaining, got, tc.want)
			}
		})
	}
}

// TestBoundedWaitWithoutAWalkIsUnbounded pins the direct-call case: a call that
// carries no walk budget keeps the behavior the transport always had.
func TestBoundedWaitWithoutAWalkIsUnbounded(t *testing.T) {
	if wait, ok := boundedWait(time.Hour, time.Time{}); !ok || wait != time.Hour {
		t.Fatalf("boundedWait(hour, zero) = %v, %v, want the wait unchanged", wait, ok)
	}
}

// clampTransport builds a transport whose connector answers one server, so the
// clamp tests drive the real Do path without a registry.
func clampTransport(t *testing.T, server *httptest.Server, id string) (*Transport, registry.Provider) {
	t.Helper()
	connectors, err := provider.NewConnectors(provider.DefaultFactory, &plainConnector{
		Base: provider.Base{ID: id, Auth: "apikey", Format: registry.DefaultFormat},
		url:  server.URL,
	})
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{Connectors: connectors, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	return transport, registry.Provider{
		ID:        id,
		Transport: registry.Transport{BaseURL: server.URL, Format: registry.DefaultFormat},
	}
}

// TestTransport_HonoredRetryAfterBeyondTheWalkFailsFast pins the live issue: an
// upstream answering 429 with a Retry-After of minutes inside a walk of seconds
// is reported now, as the rate limit it is, rather than after the sleep the
// header asked for. The retry is not started because it could not finish inside
// the walk, and the real cause reaches the caller instead of a masked timeout.
func TestTransport_HonoredRetryAfterBeyondTheWalkFailsFast(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"the free tier is rate limited"}}`))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "th-harbor")
	budget := &RequestBudget{deadline: time.Now().Add(2 * time.Second), total: 30 * time.Second, attempts: 8}

	started := time.Now()
	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "deepseek-v4.1-flash:free"},
		Body: []byte(`{"model":"deepseek-v4.1-flash:free"}`), Budget: budget,
	})
	elapsed := time.Since(started)

	failure, ok := AsUpstreamError(err)
	if !ok || failure.Status != http.StatusTooManyRequests {
		t.Fatalf("Do() error = %v, want the upstream's 429", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1: a wait that cannot fit in the walk must not be started", got)
	}
	if elapsed > time.Second {
		t.Fatalf("Do() took %v, want the failure reported without the honored wait", elapsed)
	}
}

// TestTransport_HonoredRetryAfterInsideTheWalkIsSlept pins the other half of
// the clamp: a wait the walk can afford is still honored, so the fix narrows
// nothing that fits.
func TestTransport_HonoredRetryAfterInsideTheWalkIsSlept(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "patient")
	budget := &RequestBudget{deadline: time.Now().Add(10 * time.Second), total: 30 * time.Second, attempts: 8}

	upstream, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m"}`), Budget: budget,
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want the retry the walk can afford", err)
	}
	defer func() {
		// reason: the body is drained here; a close error is not actionable.
		_ = upstream.Body.Close()
	}()
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want the honored retry to happen", got)
	}
}

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
