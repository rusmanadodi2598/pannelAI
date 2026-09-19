//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/oauth_state_test.go
// @for       Integration tests for the OAuth state store: staging, the
//
//	single-use take, the replay answer, and the TTL bound.
//
// @uses      github.com/redis/go-redis/v9, context, errors, os, testing, time,
//
//	internal/repository.
//
// @reason    G14 in the P2 register: the store had no test at all, so the one
//
//	command it depends on went unnoticed until a live callback answered
//	500. The replay guard is a security rule (SPEC-API-001 §4), so its
//	exact behaviour is pinned against a real server: one take wins, a
//	second gets the documented replay answer, a colliding stage is
//	refused without replacing the first flow's verifier, and an
//	abandoned state expires on its own.
//
//	The file carries an `integration` build tag, so the default
//	`go test ./...` stays hermetic on a machine with no Redis (AGENTS.md §2.1
//	forbids t.Skip as a way to sidestep a test). With the tag active the
//	address is required, not optional: a missing value fails the test
//	rather than passing silently.
//
//	  PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port' \
//	    go test -race -tags=integration ./internal/repository/redis/
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package redisrepo

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// newTestStateStore connects to the configured server and returns the store
// plus the client, so a test can inspect the raw key.
func newTestStateStore(t *testing.T) (*OAuthStateStore, *redis.Client) {
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

// cleanupState removes one staged state after the test, so a failure mid-test
// leaves nothing behind for the next run to trip over.
func cleanupState(t *testing.T, client *redis.Client, state string) string {
	t.Helper()
	key := oauthStateKey(state)
	t.Cleanup(func() {
		if err := client.Del(context.Background(), key).Err(); err != nil {
			t.Errorf("cleaning up %s: %v", key, err)
		}
	})
	return key
}

// TestOAuthStateStore_SingleUse pins the replay guard end to end: the state is
// staged with a TTL, the first take returns the payload and deletes the key, and
// the second take is the documented replay answer rather than an error.
func TestOAuthStateStore_SingleUse(t *testing.T) {
	store, client := newTestStateStore(t)
	ctx := context.Background()
	state := "test-single-use"
	key := cleanupState(t, client, state)
	payload := []byte(`{"verifier":"v1"}`)

	if err := store.Stage(ctx, state, payload, time.Minute); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	// The staged key carries a TTL, so an abandoned flow cannot sit forever.
	if ttl := client.TTL(ctx, key).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL = %v, want a positive value within the staged minute", ttl)
	}

	got, ok, err := store.Take(ctx, state)
	if err != nil {
		t.Fatalf("Take() error = %v", err)
	}
	if !ok || string(got) != string(payload) {
		t.Fatalf("Take() = (%q, %v), want (%q, true)", got, ok, payload)
	}
	if exists := client.Exists(ctx, key).Val(); exists != 0 {
		t.Fatal("the key still exists after Take(), want it deleted")
	}

	again, ok, err := store.Take(ctx, state)
	if err != nil {
		t.Fatalf("second Take() error = %v, want the replay answer without an error", err)
	}
	if ok || again != nil {
		t.Fatalf("second Take() = (%q, %v), want (nil, false)", again, ok)
	}
}

// TestOAuthStateStore_StageRefusesACollision pins SET NX: a second stage of the
// same state is refused and the first flow's payload survives, so a guessed
// collision cannot swap the PKCE verifier.
func TestOAuthStateStore_StageRefusesACollision(t *testing.T) {
	store, client := newTestStateStore(t)
	ctx := context.Background()
	state := "test-collision"
	cleanupState(t, client, state)

	if err := store.Stage(ctx, state, []byte("first"), time.Minute); err != nil {
		t.Fatalf("first Stage() error = %v", err)
	}
	if err := store.Stage(ctx, state, []byte("second"), time.Minute); !errors.Is(err, repository.ErrStateAlreadyStaged) {
		t.Fatalf("second Stage() error = %v, want ErrStateAlreadyStaged", err)
	}

	got, ok, err := store.Take(ctx, state)
	if err != nil || !ok {
		t.Fatalf("Take() = (%q, %v, %v), want the first payload", got, ok, err)
	}
	if string(got) != "first" {
		t.Fatalf("payload = %q, want the first flow's", got)
	}
}

// TestOAuthStateStore_TakeUnknownStateIsNotAnError pins the replay answer for a
// state that never existed: ok=false without an error, because the caller turns
// that into the callback's 400 rather than a 500.
func TestOAuthStateStore_TakeUnknownStateIsNotAnError(t *testing.T) {
	store, client := newTestStateStore(t)
	state := "test-unknown"
	cleanupState(t, client, state)

	payload, ok, err := store.Take(context.Background(), state)
	if err != nil {
		t.Fatalf("Take() of an unknown state error = %v, want none", err)
	}
	if ok || payload != nil {
		t.Fatalf("Take() = (%q, %v), want (nil, false)", payload, ok)
	}
}

// TestOAuthStateStore_TTLExpires pins the bound: a state staged with a short TTL
// disappears on its own, so a flow the browser abandoned cannot be replayed ten
// minutes later.
func TestOAuthStateStore_TTLExpires(t *testing.T) {
	store, client := newTestStateStore(t)
	ctx := context.Background()
	state := "test-expiry"
	key := cleanupState(t, client, state)

	if err := store.Stage(ctx, state, []byte("short"), 40*time.Millisecond); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for client.Exists(ctx, key).Val() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the staged state did not expire within five seconds")
		}
		time.Sleep(10 * time.Millisecond)
	}

	payload, ok, err := store.Take(ctx, state)
	if err != nil {
		t.Fatalf("Take() after expiry error = %v", err)
	}
	if ok || payload != nil {
		t.Fatalf("Take() after expiry = (%q, %v), want (nil, false)", payload, ok)
	}
}
