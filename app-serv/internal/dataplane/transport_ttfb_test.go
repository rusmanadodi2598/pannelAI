// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_ttfb_test.go
// @for       The first-byte cap: a streamed attempt that stays silent is cut, retried under the existing rules, and never repeated once it has spoken.
// @uses      context, io, net/http, net/http/httptest, sync/atomic, testing, time, internal/registry.
// @reason    A streamed attempt carried no total deadline, so an upstream that accepted the call and said nothing held the request until the client gave up (60-130s observed). The cap is the fix, and its boundary is what these tests pin: it cuts a silent stream, it leaves a speaking one alone, it retries the silence like any transient failure before any byte was piped, and it never applies to a non-streamed call.
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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// silentStreamUpstream accepts the call, sends the response headers, and never
// writes a byte of the body, which is the state a throttled session leaves a
// streamed request in.
func silentStreamUpstream(t *testing.T, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	return server
}

// TestTransport_FirstByteCapCutsASilentStream pins the cut: no byte within the
// cap fails the attempt as an upstream timeout, and the failure is retryable
// under the existing rules, so the POST cap allows exactly one repeat.
func TestTransport_FirstByteCapCutsASilentStream(t *testing.T) {
	var calls atomic.Int64
	server := silentStreamUpstream(t, &calls)

	transport, entry := clampTransport(t, server, "silent")
	entry.Transport.FirstByteTimeoutMS = 150

	started := time.Now()
	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m","stream":true}`), Stream: true,
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Do() error = nil, want the silent stream cut")
	}
	if code := AsError(err).Code; code != CodeUpstreamTimeout {
		t.Fatalf("code = %s, want %s", code, CodeUpstreamTimeout)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want one retry inside the POST cap", got)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Do() took %v, want the cap to cut each attempt, not the client to wait", elapsed)
	}
}

// TestTransport_StreamThatSpeaksIsNotRetried pins the boundary the retry rule
// turns on: once a byte has arrived the stream is committed to that upstream,
// so a stall after it is the idle guard's to report and the call is never
// repeated.
func TestTransport_StreamThatSpeaksIsNotRetried(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "speaks")
	entry.Transport.FirstByteTimeoutMS = 150
	entry.Transport.StallTimeoutMS = 150

	upstream, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m","stream":true}`), Stream: true,
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want the answer the upstream began", err)
	}

	head := make([]byte, 10)
	_, headErr := io.ReadFull(upstream.Body, head)
	// The stream now stalls. The idle guard reports it on the body; the
	// transport must not have started another call behind the caller's back.
	_, stallErr := io.ReadAll(upstream.Body)
	// reason: the body was read to its failure; a close error is not actionable.
	_ = upstream.Body.Close()

	if headErr != nil {
		t.Fatalf("reading the first frame: %v", headErr)
	}
	if string(head) != "data: {}\n\n" {
		t.Fatalf("first frame = %q, want the upstream's own bytes", head)
	}
	if stallErr == nil {
		t.Fatal("reading a stalled stream = nil error, want the idle guard's report")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want the committed stream never repeated", got)
	}
}

// TestTransport_FirstByteCapDoesNotBoundANonStreamedCall pins the scope: a
// single-body answer legitimately takes as long as the model needs, so the cap
// applies only to a streamed attempt.
func TestTransport_FirstByteCapDoesNotBoundANonStreamedCall(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "unary")
	entry.Transport.FirstByteTimeoutMS = 100

	upstream, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"}, Body: []byte(`{"model":"m"}`),
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want a non-streamed answer left to its total deadline", err)
	}
	// reason: the body is drained here; a close error is not actionable.
	_, _ = io.Copy(io.Discard, upstream.Body)
	_ = upstream.Body.Close()
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want the slow non-streamed answer served once", got)
	}
}

// TestTransport_FirstByteCapIsClampedByTheWalk pins that the silent wait is
// part of the walk's pre-commit phase: a cap longer than what the walk has left
// is tightened to it, so one silent credential cannot outlive the request.
func TestTransport_FirstByteCapIsClampedByTheWalk(t *testing.T) {
	var calls atomic.Int64
	server := silentStreamUpstream(t, &calls)

	transport, entry := clampTransport(t, server, "silent-walk")
	entry.Transport.FirstByteTimeoutMS = 10_000
	budget := &RequestBudget{deadline: time.Now().Add(300 * time.Millisecond), total: 30 * time.Second, attempts: 8}

	started := time.Now()
	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m","stream":true}`), Stream: true, Budget: budget,
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Do() error = nil, want the walk's clock to cut the silent wait")
	}
	if code := AsError(err).Code; code != CodeUpstreamTimeout {
		t.Fatalf("code = %s, want %s", code, CodeUpstreamTimeout)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Do() took %v, want the walk's 300ms to bound the wait, not the cap's 10s", elapsed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want no retry once the walk has no clock left", got)
	}
}

// TestTransport_FirstByteCapReportsTheWaitItEnforced pins the message a
// degraded-path reader sees: it names the cap, so a support log can tell a
// silent upstream from a slow one.
func TestTransport_FirstByteCapReportsTheWaitItEnforced(t *testing.T) {
	var calls atomic.Int64
	server := silentStreamUpstream(t, &calls)

	transport, entry := clampTransport(t, server, "silent-message")
	entry.Transport.FirstByteTimeoutMS = 120

	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m","stream":true}`), Stream: true,
	})
	if err == nil {
		t.Fatal("Do() error = nil, want the silent stream cut")
	}
	if !strings.Contains(err.Error(), "120ms") {
		t.Fatalf("error = %v, want it to name the enforced first-byte cap", err)
	}
}
