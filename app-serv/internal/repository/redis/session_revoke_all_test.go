//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/session_revoke_all_test.go
// @for       The password change's sign-out: every session created before RevokeAll stops authenticating.
// @uses      github.com/redis/go-redis/v9, context, os, testing, time.
// @reason    R12 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: ChangePassword left every other session alive, so a rotated credential kept its old holders signed in. The epoch comparison is storage behaviour, and only a live server can prove the key layout this package writes, per this package's tagging rule.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-03
package redisrepo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// sessionRevokeAllFixture returns a session store over a flushed Redis, so the
// test reads exactly what it seeded.
func sessionRevokeAllFixture(t *testing.T) (*SessionStore, context.Context) {
	t.Helper()
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the session store tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("closing redis client: %v", err)
		}
	})
	ctx := context.Background()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushing Redis: %v", err)
	}
	return NewSessionStore(client), ctx
}

// TestSessionStore_RevokeAllSignsOutEverySession pins that a password change's
// bulk revoke leaves no digest alive, whatever its position in scan order: the
// newest sessions a fresh attacker is most likely to hold must go with the
// oldest.
func TestSessionStore_RevokeAllSignsOutEverySession(t *testing.T) {
	store, ctx := sessionRevokeAllFixture(t)
	ttl := time.Hour

	epoch, err := store.Epoch(ctx)
	if err != nil {
		t.Fatalf("Epoch() error = %v", err)
	}
	const seeded = 40
	digests := make([]string, 0, seeded)
	for index := 0; index < seeded; index++ {
		digest := fmt.Sprintf("digest-%03d", index)
		if err := store.Create(ctx, digest, ttl, epoch); err != nil {
			t.Fatalf("Create(%s) error = %v", digest, err)
		}
		digests = append(digests, digest)
	}

	if err := store.RevokeAll(ctx); err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}
	for _, digest := range digests {
		active, err := store.Exists(ctx, digest)
		if err != nil {
			t.Fatalf("Exists(%s) error = %v", digest, err)
		}
		if active {
			t.Fatalf("session %s survived RevokeAll, want every digest invalid", digest)
		}
	}
}

// TestSessionStore_SessionCreatedAtTheNewEpochAuthenticates pins the half the
// sign-out must not break: a login that observes the epoch after the rotation
// keeps working.
func TestSessionStore_SessionCreatedAtTheNewEpochAuthenticates(t *testing.T) {
	store, ctx := sessionRevokeAllFixture(t)
	if err := store.Create(ctx, "before", time.Hour, 0); err != nil {
		t.Fatalf("Create(before) error = %v", err)
	}
	if err := store.RevokeAll(ctx); err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}
	epoch, err := store.Epoch(ctx)
	if err != nil || epoch != 1 {
		t.Fatalf("Epoch() = %d, %v, want 1", epoch, err)
	}
	if err := store.Create(ctx, "after", time.Hour, epoch); err != nil {
		t.Fatalf("Create(after) error = %v", err)
	}
	before, err := store.Exists(ctx, "before")
	if err != nil || before {
		t.Fatalf("Exists(before) = %v, %v, want false", before, err)
	}
	after, err := store.Exists(ctx, "after")
	if err != nil || !after {
		t.Fatalf("Exists(after) = %v, %v, want true", after, err)
	}
}
