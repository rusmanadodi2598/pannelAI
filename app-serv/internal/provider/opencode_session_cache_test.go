// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session_cache_test.go
// @for       The remembered-id store behind one OpenCode session and one request id per identity.
// @uses      fmt, strconv, sync, testing, time.
// @reason    The store is the new part of this design: an id whose prefix is a clock reading cannot be a pure function of the endpoint any more, so it has to be held, bounded, and shared by concurrent requests. These tests drive a clock the test owns, which is the only way to pin a TTL and a per-millisecond sequence without waiting on real time, and the race case is what pins that sharing.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// testClock is a clock a test drives forward by hand.
type testClock struct {
	mu sync.Mutex
	ms int64
}

func (c *testClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ms += by.Milliseconds()
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.UnixMilli(c.ms)
}

// minting records what it was asked to mint, so a test can tell a reused entry
// from a fresh one without reading the id format.
type minting struct {
	mu    sync.Mutex
	calls []string
}

func (m *minting) fn(now time.Time, counter uint64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	value := fmt.Sprintf("%d:%d", now.UnixMilli(), counter)
	m.calls = append(m.calls, value)
	return value
}

func (m *minting) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func newTestCache() (*openCodeIDCache, *testClock, *minting) {
	clock := &testClock{ms: 1_700_000_000_000}
	mint := &minting{}
	return &openCodeIDCache{now: clock.now, entries: make(map[string]openCodeIDEntry)}, clock, mint
}

func TestOpenCodeIDCache_ReusesOneEntryPerKey(t *testing.T) {
	cache, _, mint := newTestCache()
	first := cache.getOrMint("a", mint.fn)
	second := cache.getOrMint("a", mint.fn)
	if first != second || mint.count() != 1 {
		t.Fatalf("key a minted %d times (%s then %s), want the entry reused", mint.count(), first, second)
	}
}

func TestOpenCodeIDCache_SameMillisecondMintsDistinctSequences(t *testing.T) {
	cache, _, mint := newTestCache()
	first := cache.getOrMint("a", mint.fn)
	second := cache.getOrMint("b", mint.fn)
	if first == second {
		t.Fatalf("two keys minted inside one millisecond collided: %s", first)
	}
	if !strings.HasSuffix(second, ":2") {
		t.Fatalf("key b got %q, want the sequence to continue to 2 within the same millisecond", second)
	}
}

func TestOpenCodeIDCache_SequenceRestartsOnANewMillisecond(t *testing.T) {
	cache, clock, _ := newTestCache()
	cache.getOrMint("a", (&minting{}).fn)
	clock.advance(time.Millisecond)
	got := cache.getOrMint("b", (&minting{}).fn)
	if !strings.HasSuffix(got, ":1") {
		t.Fatalf("a fresh millisecond continued the sequence: %s", got)
	}
}

func TestOpenCodeIDCache_AgedEntryIsMintedAgain(t *testing.T) {
	cache, clock, mint := newTestCache()
	cache.getOrMint("a", mint.fn)
	clock.advance(openCodeIDCacheTTL + time.Minute)
	if got := cache.getOrMint("a", mint.fn); mint.count() != 2 || got == "" {
		t.Fatalf("an entry aged past the TTL was still reused (%q, %d mints)", got, mint.count())
	}
}

func TestOpenCodeIDCache_FullStoreEvictsTheLeastUsed(t *testing.T) {
	cache, clock, mint := newTestCache()
	for index := 0; index < openCodeIDCacheLimit; index++ {
		cache.getOrMint(fmt.Sprintf("key-%d", index), mint.fn)
		clock.advance(time.Millisecond)
	}
	if len(cache.entries) != openCodeIDCacheLimit {
		t.Fatalf("store holds %d entries, want the bound of %d", len(cache.entries), openCodeIDCacheLimit)
	}
	before := mint.count()
	// The bound is paid at the next new identity, not on a hit: touching a
	// remembered key must not cost anything.
	cache.getOrMint("one-past-the-bound", mint.fn)
	if len(cache.entries) != openCodeIDCacheLimit {
		t.Fatalf("store grew to %d entries past its bound", len(cache.entries))
	}
	if got := cache.getOrMint("key-0", mint.fn); mint.count() != before+2 {
		t.Fatalf("the oldest entry survived the bound (%q, %d mints so far)", got, mint.count())
	}
}

func TestOpenCodeIDCache_SweepDropsExpiredEntries(t *testing.T) {
	cache, clock, mint := newTestCache()
	cache.getOrMint("a", mint.fn)
	clock.advance(openCodeIDCacheTTL + openCodeIDCacheSweep)
	cache.getOrMint("b", mint.fn)
	if _, kept := cache.entries["a"]; kept {
		t.Fatal("the sweep left an entry aged past its TTL in the store")
	}
}

// TestOpenCodeIDCache_ConcurrentMintsStayRaceFree is the AGENTS.md §2.1 case for
// state the connector now shares: a connector is built once and used by
// concurrent requests, so half the goroutines contend on one key while the other
// half walks past the bound and forces evictions on the same map.
func TestOpenCodeIDCache_ConcurrentMintsStayRaceFree(t *testing.T) {
	cache, clock, mint := newTestCache()
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			// reason: a panic in a goroutine boundary has to be logged, not lost,
			// and this one is a test goroutine the race detector is watching.
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("worker %d panicked: %v", worker, r)
				}
			}()
			for call := 0; call < 200; call++ {
				key := "shared"
				if worker%2 == 1 {
					key = strconv.Itoa(worker) + "-" + strconv.Itoa(call)
				}
				if got := cache.getOrMint(key, mint.fn); got == "" {
					t.Errorf("worker %d minted an empty id for %q", worker, key)
				}
				clock.advance(time.Microsecond)
			}
		}(worker)
	}
	group.Wait()
	if len(cache.entries) > openCodeIDCacheLimit {
		t.Fatalf("concurrent mints grew the store to %d, past its bound of %d",
			len(cache.entries), openCodeIDCacheLimit)
	}
}
