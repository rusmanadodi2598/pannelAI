//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/oauth_state_peek_test.go
// @for       Integration tests for the polled read of the OAuth state store.
// @uses      github.com/redis/go-redis/v9, context, os, testing, time, internal/repository.
// @reason    A device flow re-reads its staged context on every poll, so the whole design rests on one property the callback never needed: a peek must not consume, must not clear the TTL, and must not extend it either. That is a `GET` against a key written with `SET NX … EX`, and the only way to know the driver behaves as read on a real server is to ask a real server.
//
//	PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	  go test -race -tags=integration ./internal/repository/redis/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-27
package redisrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// newPeekStateStore connects to the configured server; the address is required
// so a tagged run without one fails instead of passing quietly.
func newPeekStateStore(t *testing.T) (*OAuthStateStore, *redis.Client) {
	t.Helper()
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the OAuth state store tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("closing redis client: %v", err)
		}
	})
	return NewOAuthStateStore(client), client
}

// TestOAuthStateStore_PeekLeavesTheRoundAlive pins the polled flow's contract:
// the same payload is readable until something consumes it, and the expiry the
// stage chose survives every read.
func TestOAuthStateStore_PeekLeavesTheRoundAlive(t *testing.T) {
	store, client := newPeekStateStore(t)
	ctx := context.Background()
	state := "test-peek-alive"
	key := cleanupState(t, client, state)
	payload := []byte(`{"nonce":"n1"}`)

	if err := store.Stage(ctx, state, payload, time.Minute); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	before := client.TTL(ctx, key).Val()

	for attempt := 0; attempt < 3; attempt++ {
		got, ok, err := store.Peek(ctx, state)
		if err != nil || !ok || string(got) != string(payload) {
			t.Fatalf("Peek() #%d = (%q, %v, %v), want the staged payload", attempt, got, ok, err)
		}
	}
	if ttl := client.TTL(ctx, key).Val(); ttl <= 0 || ttl > before {
		t.Fatalf("TTL after three peeks = %v, want it untouched from %v", ttl, before)
	}

	taken, ok, err := store.Take(ctx, state)
	if err != nil || !ok || string(taken) != string(payload) {
		t.Fatalf("Take() after Peek() = (%q, %v, %v), want the payload once", taken, ok, err)
	}
	if _, ok, _ := store.Peek(ctx, state); ok {
		t.Fatal("Peek() after Take() still reported a round")
	}
}

// TestOAuthStateStore_PeekMissingRoundIsNotAnError pins the answer a poll gets
// for a code the gateway never minted, already spent, or expired: ok=false with
// no error, which the route turns into its 400.
func TestOAuthStateStore_PeekMissingRoundIsNotAnError(t *testing.T) {
	store, client := newPeekStateStore(t)
	ctx := context.Background()
	state := "test-peek-missing"
	key := cleanupState(t, client, state)

	if err := store.Stage(ctx, state, []byte("short"), 40*time.Millisecond); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for client.Exists(ctx, key).Val() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the staged round did not expire within five seconds")
		}
		time.Sleep(10 * time.Millisecond)
	}

	payload, ok, err := store.Peek(ctx, state)
	if err != nil {
		t.Fatalf("Peek() after expiry error = %v, want none", err)
	}
	if ok || payload != nil {
		t.Fatalf("Peek() after expiry = (%q, %v), want (nil, false)", payload, ok)
	}
}
