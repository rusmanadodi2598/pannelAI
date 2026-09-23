// Command app-serv adapts a provider node's model list to HTTP.
//
// @file      cmd/app-serv/node_models_cache_test.go
// @for       The cache window: one upstream read per node per TTL, not one per
//
//	lookup.
//
// @uses      internal/netguard, internal/provider, internal/service, context,
//
//	net/http, net/http/httptest, sync/atomic, testing, time.
//
// @reason    The provider index overlay that calls this adapter rebuilds on every
// //
//
//	Provider/All lookup (cmd/app-serv/provider_index.go), and one catalog
//	request walks the whole index. Without a cache, rendering one panel
//	page would dial every custom node's upstream once per row. The window
//	is asserted with an injected clock so the test states the TTL rather
//	than sleeping through it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// countingNodeServer is a node upstream that answers a model list and counts how
// many times it was asked.
type countingNodeServer struct {
	server *httptest.Server
	hits   atomic.Int64
	status atomic.Int64
}

func newCountingNodeServer(t *testing.T) *countingNodeServer {
	t.Helper()
	c := &countingNodeServer{}
	c.status.Store(http.StatusOK)
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		c.hits.Add(1)
		w.WriteHeader(int(c.status.Load()))
		_, _ = w.Write([]byte(`{"data":[{"id":"cached-model"}]}`))
	}))
	t.Cleanup(c.server.Close)
	return c
}

// cacheFixture is one adapter over a counting server, with a controllable clock.
type cacheFixture struct {
	source *nodeModelSource
	server *countingNodeServer
	now    time.Time
}

func newCacheFixture(t *testing.T, successTTL time.Duration) *cacheFixture {
	t.Helper()
	fixture := &cacheFixture{server: newCountingNodeServer(t), now: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)}
	guard, err := netguard.NewGuard([]string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		t.Fatalf("netguard.NewGuard() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		t.Fatalf("provider.NewConnectors() error = %v", err)
	}
	fixture.source = newNodeModelSource(
		staticNodeLookup{id: "openai-compatible-01TEST", baseURL: fixture.server.server.URL, format: "openai", apiType: "chat"}.lookup,
		connectors, guard, staticCredential("sk-live-abcdef"), successTTL,
	)
	fixture.source.clock = func() time.Time { return fixture.now }
	return fixture
}

// TestNodeModelSource_CachesWithinTTL is the table the finding names: a miss, a
// hit inside the window, and a miss after it.
func TestNodeModelSource_CachesWithinTTL(t *testing.T) {
	cases := []struct {
		name       string
		advance    time.Duration
		wantHits   int64
		wantSource string
	}{
		{name: "the first call is a miss", advance: 0, wantHits: 1, wantSource: service.ModelSourceUpstream},
		{name: "a second call inside the window is a hit", advance: 30 * time.Second, wantHits: 1, wantSource: service.ModelSourceUpstream},
		{name: "a call just before the window closes is still a hit", advance: 59 * time.Second, wantHits: 1, wantSource: service.ModelSourceUpstream},
		{name: "a call after the window is a miss again", advance: 61 * time.Second, wantHits: 2, wantSource: service.ModelSourceUpstream},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newCacheFixture(t, time.Minute)
			// Warm the cache, then advance to the case's instant.
			if _, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST"); err != nil {
				t.Fatalf("warming the cache: %v", err)
			}
			fixture.now = fixture.now.Add(tc.advance)
			list, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
			if err != nil {
				t.Fatalf("ListNodeModels() error = %v", err)
			}
			if list.Source != tc.wantSource {
				t.Fatalf("Source = %q, want %q", list.Source, tc.wantSource)
			}
			if got := fixture.server.hits.Load(); got != tc.wantHits {
				t.Fatalf("the upstream was asked %d times, want %d", got, tc.wantHits)
			}
		})
	}
}

// TestNodeModelSource_FailedFetchIsCachedBriefly pins that a down upstream is
// not hammered: the fallback is remembered for the short window, and asking
// again inside it costs nothing.
func TestNodeModelSource_FailedFetchIsCachedBriefly(t *testing.T) {
	fixture := newCacheFixture(t, time.Minute)
	fixture.server.status.Store(http.StatusInternalServerError)

	first, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
	if err != nil {
		t.Fatalf("ListNodeModels() error = %v", err)
	}
	if first.Source != service.ModelSourceRegistry || first.Warning == "" {
		t.Fatalf("a failing upstream did not produce a warning fallback: %+v", first)
	}
	// Inside the failure window: no second request.
	fixture.now = fixture.now.Add(nodeModelFailureTTL / 2)
	second, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
	if err != nil {
		t.Fatalf("ListNodeModels() error = %v", err)
	}
	if second.Warning != first.Warning {
		t.Fatalf("the remembered warning changed: %q then %q", first.Warning, second.Warning)
	}
	if got := fixture.server.hits.Load(); got != 1 {
		t.Fatalf("a failing upstream was asked %d times inside the failure window, want 1", got)
	}
	// After the failure window the upstream is tried again, so a recovery is
	// noticed rather than cached away.
	fixture.now = fixture.now.Add(nodeModelFailureTTL)
	fixture.server.status.Store(http.StatusOK)
	third, err := fixture.source.ListNodeModels(context.Background(), "openai-compatible-01TEST")
	if err != nil {
		t.Fatalf("ListNodeModels() error = %v", err)
	}
	if third.Source != service.ModelSourceUpstream {
		t.Fatalf("Source after the failure window = %q, want %q once the upstream recovered", third.Source, service.ModelSourceUpstream)
	}
	if got := fixture.server.hits.Load(); got != 2 {
		t.Fatalf("the upstream was asked %d times, want 2", got)
	}
}

// TestNodeModelSource_UnknownNodeIsRemembered pins that an id the lookup does
// not resolve still caches: a stale provider_id in a request must not turn into
// a repeated store read.
func TestNodeModelSource_UnknownNodeIsRemembered(t *testing.T) {
	fixture := newCacheFixture(t, time.Minute)
	list, err := fixture.source.ListNodeModels(context.Background(), "no-such-node")
	if err != nil {
		t.Fatalf("ListNodeModels() error = %v", err)
	}
	if list.Source != service.ModelSourceRegistry || list.Warning == "" {
		t.Fatalf("an unknown node did not produce a warning fallback: %+v", list)
	}
	if got := fixture.server.hits.Load(); got != 0 {
		t.Fatalf("an unknown node dialed an upstream %d times", got)
	}
}
