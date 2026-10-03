// Command app-serv keeps the runtime provider overlay for a short window.
//
// @file      cmd/app-serv/provider_index_cache_test.go
// @for       The overlay cache: one node read per TTL window, the last good
//
//	overlay served through a store failure, and invalidation forcing a
//	rebuild the moment a node write lands.
//
// @uses      internal/domain, internal/registry, context, sync, testing, time.
// @reason    The overlay was rebuilt on every lookup — one full node query plus
//
//	a registry rebuild per request (draft 042 R06). The cache must still
//	serve a node the operator just created (the adapter's founding
//	property), so the window is asserted with an injected clock and the
//	write path is pinned to invalidate.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-10-03
package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// countingNodeLister serves nodes and counts how often the store was read, so a
// test can tell a cached overlay from a rebuilt one.
type countingNodeLister struct {
	mu    sync.Mutex
	nodes []domain.ProviderNode
	calls int
	err   error
}

func (s *countingNodeLister) List(context.Context) ([]domain.ProviderNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.nodes, nil
}

func (s *countingNodeLister) listCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// cacheClock is a clock the test moves by hand.
type cacheClock struct{ now time.Time }

func (c *cacheClock) Now() time.Time { return c.now }

// TestRuntimeProviderIndex_OverlayIsCachedForTheTTLWindow keeps one lookup
// window to one node read: a second resolution inside the window must not touch
// the store again, and one past the window must.
func TestRuntimeProviderIndex_OverlayIsCachedForTheTTLWindow(t *testing.T) {
	clock := &cacheClock{now: time.Date(2026, 10, 3, 14, 0, 0, 0, time.UTC)}
	lister := &countingNodeLister{nodes: []domain.ProviderNode{storedNode(t, domain.NodeOpenAICompatible, domain.NodeAPIChat)}}
	index := newRuntimeProviderIndex(embeddedIndex(t), lister, nil, nil)
	index.clock = clock.Now

	if _, ok := index.Provider("mycorp"); !ok {
		t.Fatal("Provider(mycorp) = not found, want the custom node")
	}
	if got := lister.listCalls(); got != 1 {
		t.Fatalf("node store reads after the first lookup = %d, want 1", got)
	}

	clock.now = clock.now.Add(overlayCacheTTL / 2)
	if _, ok := index.Provider("mycorp"); !ok {
		t.Fatal("Provider(mycorp) = not found inside the window, want the cached overlay")
	}
	if got := lister.listCalls(); got != 1 {
		t.Fatalf("node store reads inside the window = %d, want 1 (the overlay must be cached)", got)
	}

	clock.now = clock.now.Add(overlayCacheTTL)
	if _, ok := index.Provider("mycorp"); !ok {
		t.Fatal("Provider(mycorp) = not found past the window, want a rebuilt overlay")
	}
	if got := lister.listCalls(); got != 2 {
		t.Fatalf("node store reads past the window = %d, want 2 (the window must expire)", got)
	}
}

// TestRuntimeProviderIndex_LoadFailureServesTheLastOverlay keeps a store outage
// from unlisting every custom node: once an overlay is built, a failed read
// serves it rather than falling back to the embedded registry alone.
func TestRuntimeProviderIndex_LoadFailureServesTheLastOverlay(t *testing.T) {
	clock := &cacheClock{now: time.Date(2026, 10, 3, 14, 0, 0, 0, time.UTC)}
	lister := &countingNodeLister{nodes: []domain.ProviderNode{storedNode(t, domain.NodeOpenAICompatible, domain.NodeAPIChat)}}
	index := newRuntimeProviderIndex(embeddedIndex(t), lister, nil, nil)
	index.clock = clock.Now

	if _, ok := index.Provider("mycorp"); !ok {
		t.Fatal("Provider(mycorp) = not found, want the custom node")
	}

	lister.mu.Lock()
	lister.err = errors.New("the node table is briefly unavailable")
	lister.mu.Unlock()
	clock.now = clock.now.Add(overlayCacheTTL)

	if _, ok := index.Provider("mycorp"); !ok {
		t.Fatal("Provider(mycorp) = not found during a store outage, want the last good overlay")
	}
	if _, ok := index.Provider(nodeIDOf(t, lister)); !ok {
		t.Fatal("Provider(node id) = not found during a store outage, want the last good overlay")
	}
}

// nodeIDOf reads the id of the first node the lister serves, so the test
// resolves by the id the panel stores rather than only by the prefix.
func nodeIDOf(t *testing.T, lister *countingNodeLister) string {
	t.Helper()
	lister.mu.Lock()
	defer lister.mu.Unlock()
	if len(lister.nodes) == 0 {
		t.Fatal("countingNodeLister serves no nodes")
	}
	return lister.nodes[0].ID()
}

// TestRuntimeProviderIndex_InvalidateForcesARebuild makes the write path honest:
// a node the operator just created must be resolvable immediately, not after the
// window closes.
func TestRuntimeProviderIndex_InvalidateForcesARebuild(t *testing.T) {
	clock := &cacheClock{now: time.Date(2026, 10, 3, 14, 0, 0, 0, time.UTC)}
	lister := &countingNodeLister{}
	index := newRuntimeProviderIndex(embeddedIndex(t), lister, nil, nil)
	index.clock = clock.Now

	if _, ok := index.Provider("mycorp"); ok {
		t.Fatal("Provider(mycorp) = found, want an empty store to refuse it")
	}

	lister.mu.Lock()
	lister.nodes = []domain.ProviderNode{storedNode(t, domain.NodeOpenAICompatible, domain.NodeAPIChat)}
	lister.mu.Unlock()
	index.InvalidateNodeOverlay()

	if _, ok := index.Provider("mycorp"); !ok {
		t.Fatal("Provider(mycorp) = not found after Invalidate, want the rebuilt overlay")
	}
	if _, ok := index.Provider(registry.Provider{}.ID); ok {
		t.Fatal("Provider(empty name) = found, want no entry for a blank name")
	}
}
