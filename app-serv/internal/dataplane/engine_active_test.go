// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_active_test.go
// @for       The in-flight marker at the relay leg: one provider recorded while
//
//	its outbound call runs, and released on every exit.
//
// @uses      context, net/http, net/http/httptest, sync, testing, time,
//
//	internal/domain.
//
// @reason    The drawing's active node means "this provider is being called
//
//	right now" (SPEC-UI-001 §6.5), and the relay leg is the one place
//	that knows both the provider and the interval. The release has to
//	run on every exit, including a failure, or a node stays lit for a
//	request that ended; that is what these tests pin, and they drive
//	the real engine over the shared relay fixture so the exit paths are
//	the production ones.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-22
package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// activeRecorder is a double for the ActiveRequests seam. It is not a store
// double: the store's Redis behaviour is pinned against a real server elsewhere,
// and what this test needs is to observe the interval between the marker opening
// and its release.
type activeRecorder struct {
	mu        sync.Mutex
	begun     []string
	released  int
	stillOpen int
}

func (r *activeRecorder) Begin(_ context.Context, providerID, _, _ string) func() {
	r.mu.Lock()
	r.begun = append(r.begun, providerID)
	r.stillOpen++
	r.mu.Unlock()

	var once bool
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if once {
			return
		}
		once = true
		r.released++
		r.stillOpen--
	}
}

func (r *activeRecorder) snapshot() (begun []string, released, stillOpen int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.begun...), r.released, r.stillOpen
}

// newActiveEngine wires the real relay pipeline with the tracker under test. It
// builds on the shared fixture's own providers and endpoints rather than
// re-declaring them, so only the seam this file is about differs.
func newActiveEngine(t *testing.T, upstreamURL string, active ActiveRequests) *Engine {
	t.Helper()
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	return newEngineWithTracker(t, upstreamURL, repo, nil, active)
}

// newEngineWithTracker is newEngineWith plus the in-flight marker seam, so the
// tests that pin the marker's interval drive the same pipeline as every other
// engine test rather than a second wiring of their own.
func newEngineWithTracker(t *testing.T, upstreamURL string, repo *memEndpointRepo, combos map[string]domain.Combo, active ActiveRequests) *Engine {
	t.Helper()
	return newEngineFull(t, []registry.Provider{
		relayProvider("alpha", upstreamURL), relayProvider("beta", upstreamURL),
	}, repo, combos, nil, active)
}

// TestRelayOnce_MarksTheProviderWhileItRuns pins the marker's interval: it opens
// before the outbound call and is released before the leg returns, so a node is
// lit for exactly as long as the provider is being called.
func TestRelayOnce_MarksTheProviderWhileItRuns(t *testing.T) {
	active := &activeRecorder{}
	calls := 0
	server := newRelayUpstream(t, &calls)
	engine := newActiveEngine(t, server.URL, active)

	if _, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	begun, released, stillOpen := active.snapshot()
	if len(begun) != 1 {
		t.Fatalf("markers begun = %v, want exactly one", begun)
	}
	if begun[0] != "alpha" {
		t.Fatalf("the marker names provider %q, want alpha", begun[0])
	}
	if released != 1 {
		t.Fatalf("markers released = %d, want 1", released)
	}
	if stillOpen != 0 {
		t.Fatalf("%d markers are still open, want none: a finished call must not leave a node lit", stillOpen)
	}
}

// TestRelayOnce_ReleasesTheMarkerWhenTheCallFails pins the failure path: an
// upstream that refuses the request still releases its marker, because a node lit
// for a request that already failed is a claim about now that is not true.
func TestRelayOnce_ReleasesTheMarkerWhenTheCallFails(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantFail bool
	}{
		{name: "the upstream refuses the request", status: http.StatusBadRequest, wantFail: true},
		{name: "the upstream reports an internal error", status: http.StatusInternalServerError, wantFail: true},
		{name: "the upstream answers normally", status: http.StatusOK, wantFail: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active := &activeRecorder{}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.status == http.StatusOK {
					_, _ = w.Write([]byte(`{"id":"c","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`))
					return
				}
				_, _ = w.Write([]byte(`{"error":{"message":"no","type":"invalid_request_error"}}`))
			}))
			t.Cleanup(upstream.Close)

			engine := newActiveEngine(t, upstream.URL, active)
			_, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil)
			if tc.wantFail && err == nil {
				t.Fatal("Relay() = nil, want a failure")
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("Relay() = %v, want nil", err)
			}

			_, released, stillOpen := active.snapshot()
			if released != 1 || stillOpen != 0 {
				t.Fatalf("released = %d and still open = %d, want 1 and 0: every exit must release the marker",
					released, stillOpen)
			}
		})
	}
}

// TestRelayOnce_MarksEachComboMemberItAttempts pins the failover path: a combo
// that tries a second provider marks that provider too, so the drawing shows the
// provider actually being called rather than the first one it named.
func TestRelayOnce_MarksEachComboMemberItAttempts(t *testing.T) {
	active := &activeRecorder{}
	calls := 0
	server := newRelayUpstream(t, &calls)
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	repo.byProvider["beta"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-beta", "beta")}
	engine := newEngineWithTracker(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "beta/works"),
	}, active)

	if _, err := engine.Relay(context.Background(), relayRequest("daily"), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}

	begun, released, stillOpen := active.snapshot()
	if len(begun) != 2 {
		t.Fatalf("markers begun = %v, want one per attempted member", begun)
	}
	if begun[0] != "alpha" || begun[1] != "beta" {
		t.Fatalf("markers begun = %v, want alpha then beta", begun)
	}
	if released != len(begun) {
		t.Fatalf("begun = %d but released = %d: every attempted member must be released", len(begun), released)
	}
	if stillOpen != 0 {
		t.Fatalf("%d markers are still open, want none", stillOpen)
	}
}

// TestRelayOnce_NilTrackerIsANoOp pins that a deployment which wired no Redis
// serves every request without a nil check at the call site.
func TestRelayOnce_NilTrackerIsANoOp(t *testing.T) {
	calls := 0
	server := newRelayUpstream(t, &calls)
	engine := newActiveEngine(t, server.URL, nil)
	if _, err := engine.Relay(context.Background(), relayRequest("alpha/works"), nil); err != nil {
		t.Fatalf("Relay() with no tracker = %v, want nil", err)
	}
}
