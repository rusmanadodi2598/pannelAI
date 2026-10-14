// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session_cache.go
// @for       The bounded store behind one canonical OpenCode id per identity.
// @uses      sync, time.
// @reason    The free tier accounts quota per session, so minting a fresh id for every request spends the quota the request is trying to buy; the id has to be remembered per identity. Remembering it means the connector holds state it did not hold before, and a connector is built once and shared by concurrent requests, so the state needs a lock. Entries also have to stop living forever: an identity pinned to one session never rotates its quota window, which is why the reference runs a TTL sweeper over the same store.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"sync"
	"time"
)

// openCodeIDCacheLimit is the reference's MAX_STABLE_SESSIONS: one thousand
// identities, the same bound for both id kinds.
const openCodeIDCacheLimit = 1000

// openCodeIDCacheTTL and openCodeIDCacheSweep are the reference's
// MEMORY_CONFIG.sessionTtlMs and sessionCleanupIntervalMs. Carrying the numbers
// over keeps one identity's session alive across a working conversation and no
// longer than that.
const (
	openCodeIDCacheTTL   = 2 * time.Hour
	openCodeIDCacheSweep = 30 * time.Minute
)

// openCodeIDEntry is one remembered id plus the clock reading that decides
// whether it is still current and, when the store is full, which entry goes.
type openCodeIDEntry struct {
	value  string
	usedAt time.Time
}

// openCodeIDCache is the remembered-id store. `now` is injected so the TTL and
// the per-millisecond sequence are testable without waiting on a wall clock.
type openCodeIDCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]openCodeIDEntry
	lastMs  int64
	counter uint64
	lastCut time.Time
}

// newOpenCodeIDCache builds an empty store.
func newOpenCodeIDCache() *openCodeIDCache {
	return &openCodeIDCache{now: time.Now, entries: make(map[string]openCodeIDEntry)}
}

// getOrMint returns the remembered id for key, or mints one with `mint` and
// remembers it. mint receives the current clock and the sequence number this
// mint must carry, which is how two ids minted inside the same millisecond stay
// distinct.
//
// Every path through this method is a single decision under one lock: the
// sequence counter and the store are mutated together, so concurrent requests
// can never be handed the same clock-plus-sequence prefix.
func (c *openCodeIDCache) getOrMint(key string, mint func(now time.Time, counter uint64) string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	current := c.now()
	if entry, found := c.entries[key]; found && current.Sub(entry.usedAt) < openCodeIDCacheTTL {
		entry.usedAt = current
		c.entries[key] = entry
		return entry.value
	}

	c.discardExpired(current)
	c.counter = c.nextSequence(current)
	value := mint(current, c.counter)
	if len(c.entries) >= openCodeIDCacheLimit {
		c.dropLeastRecentlyUsed()
	}
	c.entries[key] = openCodeIDEntry{value: value, usedAt: current}
	return value
}

// nextSequence continues the reference's clock-pair rule: the sequence restarts
// at one whenever the millisecond changes, and increments within it.
func (c *openCodeIDCache) nextSequence(now time.Time) uint64 {
	ms := now.UnixMilli()
	if ms != c.lastMs {
		c.lastMs = ms
		c.counter = 0
	}
	return c.counter + 1
}

// discardExpired drops entries no identity has used inside the TTL. It runs at
// most once per openCodeIDCacheSweep, so the sweep costs a clock comparison on a
// warm store instead of a scan on every request: the reference pays that scan
// from a background interval, and starting one here would need a lifecycle the
// gateway does not hand a connector. The zero `lastCut` makes the first mint the
// sweep that clears whatever a previous window left behind.
func (c *openCodeIDCache) discardExpired(current time.Time) {
	if !c.lastCut.IsZero() && current.Before(c.lastCut.Add(openCodeIDCacheSweep)) {
		return
	}
	c.lastCut = current
	cutoff := current.Add(-openCodeIDCacheTTL)
	for key, entry := range c.entries {
		if entry.usedAt.Before(cutoff) {
			delete(c.entries, key)
		}
	}
}

// dropLeastRecentlyUsed frees one slot by evicting the oldest reading. The
// reference deletes the Map's first key, which is its insertion order; the two
// agree for a store that is only ever written on a miss.
func (c *openCodeIDCache) dropLeastRecentlyUsed() {
	oldest := time.Time{}
	victim := ""
	for key, entry := range c.entries {
		if victim == "" || entry.usedAt.Before(oldest) {
			oldest, victim = entry.usedAt, key
		}
	}
	if victim != "" {
		delete(c.entries, victim)
	}
}
