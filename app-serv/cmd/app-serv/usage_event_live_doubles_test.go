//go:build integration

// Package main is the app-serv composition root.
//
// @file      cmd/app-serv/usage_event_live_doubles_test.go
// @for       The live-stack adapters and the broken broker the usage event live
//
//	pass needs.
//
// @uses      internal/domain, internal/repository, internal/repository/postgres,
//
//	github.com/redis/go-redis/v9, context, testing, time.
//
// @reason    The live pass drives the real recorder and the real consumer over
//
//	the stack's real pool and Redis client. These adapters exist only
//	because the stack harness builds its own small graph rather than
//	reusing buildObservability, so the two services the pass needs are
//	rebuilt here over the same live dependencies. They are not doubles:
//	each one is the production repository constructor over the live pool.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-22
package main

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
)

// newLiveUsageRepo is the production usage repository over the stack's pool.
func newLiveUsageRepo(stack liveStack) repository.UsageRecordRepository {
	return postgres.NewUsageRepository(stack.pool)
}

// newLiveLogRepo is the production request-log repository over the stack's pool.
func newLiveLogRepo(stack liveStack) repository.RequestLogRepository {
	return postgres.NewLogRepository(stack.pool)
}

// newLiveSettingsRepo is the production settings repository over the stack's
// pool, so the consumer's console bound is read from the same table the panel's
// settings route writes.
func newLiveSettingsRepo(stack liveStack) repository.SettingsRepository {
	return postgres.NewSettingsRepository(stack.pool)
}

// brokenRedisClient returns a client pointed at a closed port, which is the
// cheapest honest way to make the broker fail while every other dependency of
// the pass stays real.
func brokenRedisClient(t *testing.T) redis.UniversalClient {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond,
		ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond,
		MaxRetries: -1,
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}
