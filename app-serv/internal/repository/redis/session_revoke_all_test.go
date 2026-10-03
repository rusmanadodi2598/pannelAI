//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/session_revoke_all_test.go
// @for       The password change's sign-out: every tracked session digest is
//
//	gone after RevokeAll.
//
// @uses      github.com/redis/go-redis/v9, context, os, testing, time.
// @reason    R12 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: ChangePassword left
//
//	every other session alive, so a rotated credential kept its old
//	holders signed in. The scan-and-delete walk is storage behaviour —
//	which keys a SCAN page names, which deletes land — and only a live
//	server can prove it, per this package's tagging rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
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

	const seeded = 40
	digests := make([]string, 0, seeded)
	for index := 0; index < seeded; index++ {
		digest := fmt.Sprintf("digest-%03d", index)
		if err := store.Create(ctx, digest, ttl); err != nil {
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
			t.Fatalf("session %s survived RevokeAll, want every digest gone", digest)
		}
	}
}
