// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/failure_classification_test.go
// @for       The transport's read of one upstream refusal: no same-key retry for a spent allowance or a closed gate, and the retry a throttle still earns.
// @uses      context, net/http, net/http/httptest, sync/atomic, testing, time, internal/domain, internal/registry.
// @reason    C1 is one mechanism read from both ends, and this is the sending end: a 429 that is really a spent allowance must rotate instead of sleeping the walk on a credential that cannot serve. It is pinned against a stand-in upstream so the transport's own retry decision is what fails when it changes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// The two refusals TH Harbor sends under one status, and the reset the second one
// prints. Taken from the parked keys of 2026-10-15: four of six were parked for
// five minutes against a reset four days away.
const (
	harborHeavyLoad = `{"error":{"message":"Token Harbor is under heavy load right now, ` +
		`so your requests have been slowed down for a few minutes."}}`
	harborAllowanceSpent = `{"error":{"message":"You've used this period's free allowance. ` +
		`Your next period starts on 19 Oct 2026 at 10:48 UTC."}}`
	harborGate = `OpenCode's free tier can only be used from within OpenCode`
)

func harborReset(t *testing.T) time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339, "2026-10-19T10:48:00Z")
	if err != nil {
		t.Fatalf("parsing the fixture reset: %v", err)
	}
	return when
}

// TestTransport_QuotaExhaustedIsNotRetriedOnTheSameKey is half of C1: the account
// cannot pay for this request, so the call is not repeated on it. The walk moves
// on to a credential that can serve instead of spending its budget on one that
// cannot.
func TestTransport_QuotaExhaustedIsNotRetriedOnTheSameKey(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// A Retry-After short enough to fit any walk: the old code would have
		// slept it and asked the same key again.
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(harborAllowanceSpent))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "th-harbor")
	budget := &RequestBudget{deadline: time.Now().Add(30 * time.Second), total: 30 * time.Second, attempts: 8}

	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "deepseek-v4.1-flash:free"},
		Body: []byte(`{"model":"deepseek-v4.1-flash:free"}`), Budget: budget,
	})

	failure, ok := AsUpstreamError(err)
	if !ok || failure.Status != http.StatusTooManyRequests {
		t.Fatalf("Do() error = %v, want the upstream's 429", err)
	}
	if failure.Class != domain.FailureQuotaExhausted {
		t.Fatalf("class = %s, want %s", failure.Class, domain.FailureQuotaExhausted)
	}
	if !failure.ResetAt.Equal(harborReset(t)) {
		t.Fatalf("reset = %v, want the %v the body stated", failure.ResetAt, harborReset(t))
	}
	if failure.SignatureID != "tokenharbor.free-allowance" {
		t.Fatalf("signature = %q, want the allowance signature", failure.SignatureID)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1: a spent allowance must rotate, not retry", got)
	}
}

// TestTransport_AThrottleIsStillRetried pins the other half: a 429 that is a
// throttle keeps the same-key retry the transport always gave it, so narrowing
// the quota case narrows nothing else.
func TestTransport_AThrottleIsStillRetried(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(harborHeavyLoad))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "th-harbor")
	budget := &RequestBudget{deadline: time.Now().Add(30 * time.Second), total: 30 * time.Second, attempts: 8}

	upstream, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m"}`), Budget: budget,
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want the honored retry", err)
	}
	defer func() {
		// reason: the body is drained here; a close error is not actionable.
		_ = upstream.Body.Close()
	}()
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want the retry a throttle still earns", got)
	}
}

// TestTransport_AGateIsNotRetriedNorParked is the OpenCode case: the refusal is
// about this gateway, so no key on the account can serve it and no retry can
// change it.
func TestTransport_AGateIsNotRetried(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(harborGate))
	}))
	t.Cleanup(server.Close)

	transport, entry := clampTransport(t, server, "opencode")
	budget := &RequestBudget{deadline: time.Now().Add(30 * time.Second), total: 30 * time.Second, attempts: 8}

	_, err := transport.Do(context.Background(), Call{
		Provider: entry, Model: registry.Model{ID: "m"},
		Body: []byte(`{"model":"m"}`), Budget: budget,
	})

	failure, ok := AsUpstreamError(err)
	if !ok || failure.Class != domain.FailureGateBlocked {
		t.Fatalf("Do() error = %v, want the gate's own class", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1: a closed gate is not worth a second try", got)
	}
}
