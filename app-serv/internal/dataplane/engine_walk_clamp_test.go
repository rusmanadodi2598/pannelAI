// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_walk_clamp_test.go
// @for       The end-to-end outcome the clamp exists for: a provider whose keys are all rate-limited reports RATE_LIMITED in seconds, not a masked timeout after minutes.
// @uses      context, fmt, net/http, net/http/httptest, sync/atomic, testing, time, internal/domain, internal/registry.
// @reason    The live rs-agent failure: TH Harbor's six free keys were all rate-limited, each credential spent 60-130s inside the transport honoring a minutes-long Retry-After, and the walk then reported UPSTREAM_TIMEOUT instead of the RATE_LIMITED every upstream response had stated. The transport-level clamp is pinned in transport_clamp_test.go; this is the same fix observed where the client observes it, on the walk's own error.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// rateLimitedEndpoint builds one account holding the given number of keys, the
// shape a custom node with several credentials presents.
func rateLimitedEndpoint(t *testing.T, id, providerID string, keys int) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, providerID, "label-"+id, domain.UpstreamAuthAPIKey, 1, now)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint(%s) error = %v", id, err)
	}
	for index := 0; index < keys; index++ {
		key := domain.RehydrateUpstreamKey(
			fmt.Sprintf("uky-%s-%d", id, index), id, fmt.Sprintf("key-%d", index),
			fmt.Sprintf("sealed-%s-%d", id, index), "…hint",
			1, domain.UpstreamKeyActive, nil, "", 0, domain.KeyRotationState{}, nil, now, now,
			domain.OperatorState{})
		endpoint.AttachKey(key)
	}
	return endpoint
}

// TestRelay_RateLimitedWalkReportsTheRateLimit pins the desired outcome: every
// key answers 429 with an hour-long Retry-After, and the walk reports the rate
// limit quickly. Before the clamp each attempt slept the honored wait (bounded
// only by the 120s transport total), so the walk outlived its own clock and the
// client was told UPSTREAM_TIMEOUT.
func TestRelay_RateLimitedWalkReportsTheRateLimit(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"the free tier is rate limited"}}`))
	}))
	t.Cleanup(server.Close)

	repo := newMemEndpointRepo()
	repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
		rateLimitedEndpoint(t, "ep-harbor", "provider-a", 6),
	}
	engine := newEngineWith(t, []registry.Provider{relayProvider("provider-a", server.URL)}, repo,
		map[string]domain.Combo{"harbor": comboRow("harbor", "provider-a/deepseek-v4.1-flash:free")}, nil)

	started := time.Now()
	_, err := engine.Relay(context.Background(), relayRequest("harbor"), nil)
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Relay() = nil error, want the rate limit every key reported")
	}
	if code := AsError(err).Code; code != CodeRateLimited {
		t.Fatalf("code = %q, want %q: the walk must report what the upstream said, not a masked timeout", code, CodeRateLimited)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Relay() took %v, want the honored hour-long waits never slept", elapsed)
	}
	if got := calls.Load(); got != 6 {
		t.Fatalf("upstream calls = %d, want one per key with no same-target retry that cannot fit the walk", got)
	}
}
