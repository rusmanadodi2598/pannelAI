//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/key_cursor_integration_test.go
// @for       The one property the credential arbiter exists for: two callers asking at the same instant never receive the same number.
// @uses      context, fmt, os, sync, testing, time, github.com/redis/go-redis/v9
// @reason    Rotation ordering by the last success made every concurrent selection answer the same key, and no single-threaded test can see that: it is a property of interleaving. This runs against a real Redis because the guarantee is INCR's, not the wrapper's, and a fake would only prove the wrapper agrees with itself.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-14
package redisrepo

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func newKeyCursorStore(t *testing.T) (*KeyCursorStore, redis.UniversalClient) {
	t.Helper()
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the key cursor tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("closing the client: %v", err)
		}
	})
	return NewKeyCursorStore(client), client
}

func TestKeyCursor_ConcurrentCallersNeverShareAPosition(t *testing.T) {
	store, client := newKeyCursorStore(t)
	ctx := context.Background()
	endpointID := fmt.Sprintf("ep-rr-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if err := client.Del(ctx, keyCursorPrefix+cursorSegment(endpointID)).Err(); err != nil {
			t.Logf("cleaning the cursor key: %v", err)
		}
	})

	const callers = 32
	positions := make([]int, callers)
	var group sync.WaitGroup
	for i := range callers {
		group.Add(1)
		go func(slot int) {
			defer group.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("caller %d panicked: %v", slot, r)
				}
			}()
			position, err := store.Next(ctx, endpointID)
			if err != nil {
				t.Errorf("caller %d Next() error = %v", slot, err)
				return
			}
			positions[slot] = position
		}(i)
	}
	group.Wait()

	// Distinct numbers is the whole guarantee. The modulo that turns a number into
	// a key belongs to the domain, so this test proves the arbiter hands out a
	// different answer to every caller rather than the walk's use of it.
	seen := make(map[int]bool, callers)
	for _, position := range positions {
		if position <= 0 {
			t.Fatalf("a caller received %d, want a positive count", position)
		}
		if seen[position] {
			t.Fatalf("two callers received position %d, want the counter to separate every one", position)
		}
		seen[position] = true
	}
	if len(seen) != callers {
		t.Fatalf("distinct positions = %d, want %d", len(seen), callers)
	}
}

func TestKeyCursor_NamespacesOneAccountFromAnother(t *testing.T) {
	store, client := newKeyCursorStore(t)
	ctx := context.Background()
	first, second := "ep-first", "ep-second"
	t.Cleanup(func() {
		for _, id := range []string{first, second} {
			if err := client.Del(ctx, keyCursorPrefix+cursorSegment(id)).Err(); err != nil {
				t.Logf("cleaning the cursor key for %s: %v", id, err)
			}
		}
	})

	// Two accounts of the same provider are two counters: an account that just spent
	// its third key must not move which key the account beside it hands out next.
	if _, err := store.Next(ctx, first); err != nil {
		t.Fatalf("Next(first) error = %v", err)
	}
	if _, err := store.Next(ctx, first); err != nil {
		t.Fatalf("Next(first) error = %v", err)
	}
	got, err := store.Next(ctx, second)
	if err != nil {
		t.Fatalf("Next(second) error = %v", err)
	}
	if got != 1 {
		t.Fatalf("the second account started at %d, want 1: one account's walk is not another's", got)
	}
}

func TestKeyCursor_CounterExpiresOnItsOwn(t *testing.T) {
	store, client := newKeyCursorStore(t)
	ctx := context.Background()
	endpointID := "ep-ttl"
	key := keyCursorPrefix + cursorSegment(endpointID)
	t.Cleanup(func() {
		if err := client.Del(ctx, key).Err(); err != nil {
			t.Logf("cleaning the cursor key: %v", err)
		}
	})

	if _, err := store.Next(ctx, endpointID); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	// A day, not no-TTL: an account whose keys were all deleted must not leave a
	// counter behind forever, and a day is longer than any rotation cycle so expiry
	// never interrupts a live walk.
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}
	if ttl <= 0 || ttl > keyCursorTTL {
		t.Fatalf("TTL = %v, want a positive bound within %v", ttl, keyCursorTTL)
	}
}
