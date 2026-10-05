//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/vision_rotation_absent_integration_test.go
// @for       The difference between no rotation stored and Redis not answering.
// @uses      context, os, testing, github.com/redis/go-redis/v9
// @reason    The store's contract says an absent key is the zero state. Reading it as an error made
//
//	a first run indistinguishable from an outage, and the caller swallowed both, so rotation
//	restarted silently every time Redis was merely down.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-04
package redisrepo

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestVisionRotation_AbsentKeyIsTheZeroStateNotAnOutage(t *testing.T) {
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the vision rotation tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("closing the client: %v", err)
		}
	})

	store := NewVisionRotationStore(client)
	ctx := context.Background()
	// Start from a genuinely empty keyspace for this key.
	if err := client.Del(ctx, visionRotationKey()).Err(); err != nil {
		t.Fatalf("clearing the rotation key: %v", err)
	}

	state, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() with no key stored error = %v, want the zero state and no error", err)
	}
	if state != (domain.RotationState{}) {
		t.Fatalf("Get() = %+v, want the zero rotation state", state)
	}
}
