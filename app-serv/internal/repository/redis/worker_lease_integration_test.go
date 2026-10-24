//go:build integration

// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/worker_lease_integration_test.go
// @for       The single-owner worker lease and its compare-and-delete release.
// @uses      github.com/redis/go-redis/v9, context, os, testing, time.
// @reason    Two replicas must not run one worker cycle, and an expired owner must not delete the lease its successor holds; only a real Redis proves the SET NX and the Lua compare.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-22
package redisrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestWorkerLeaseIsSingleOwner(t *testing.T) {
	raw := os.Getenv(testRedisEnv)
	if raw == "" {
		t.Fatalf("%s must be set to run the worker lease tests", testRedisEnv)
	}
	client := redis.NewClient(redisOptions(t, raw))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	name := "test-lease-" + t.Name()

	first, err := NewWorkerLease(client, name, time.Minute)
	if err != nil {
		t.Fatalf("first lease: %v", err)
	}
	second, err := NewWorkerLease(client, name, time.Minute)
	if err != nil {
		t.Fatalf("second lease: %v", err)
	}

	acquired, err := first.Acquire(ctx)
	if err != nil || !acquired {
		t.Fatalf("first Acquire() = (%v, %v), want true", acquired, err)
	}
	if acquired, err := second.Acquire(ctx); err != nil || acquired {
		t.Fatalf("second Acquire() = (%v, %v), want false", acquired, err)
	}
	if err := first.Release(ctx); err != nil {
		t.Fatalf("first Release() error = %v", err)
	}
	if acquired, err := second.Acquire(ctx); err != nil || !acquired {
		t.Fatalf("second Acquire() after release = (%v, %v), want true", acquired, err)
	}
	// The first owner's release is a no-op now that the token belongs to the second.
	if err := first.Release(ctx); err != nil {
		t.Fatalf("stale Release() error = %v", err)
	}
	third, err := NewWorkerLease(client, name, time.Minute)
	if err != nil {
		t.Fatalf("third lease: %v", err)
	}
	if acquired, err := third.Acquire(ctx); err != nil || acquired {
		t.Fatalf("third Acquire() after stale release = (%v, %v), want false", acquired, err)
	}
	if err := second.Release(ctx); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}
}
