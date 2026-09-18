// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/limiter.go
// @for       Atomically counts login failures and bounds gateway request rates.
// @uses      github.com/redis/go-redis/v9, internal/repository, context, time.
// @reason    Concurrent login attempts and public requests must share durable,
//
//	bounded state instead of relying on process-local counters.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-17
package redisrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	loginFailurePrefix = "pannelai:auth:login:fail:"
	loginLockPrefix    = "pannelai:auth:login:lock:"
	ratePrefix         = "pannelai:rate:"
)

var recordFailureScript = redis.NewScript(`
local failures = redis.call("INCR", KEYS[1])
if failures == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[2])
end
if failures >= tonumber(ARGV[1]) then
  redis.call("SET", KEYS[2], "1", "EX", ARGV[2])
  redis.call("DEL", KEYS[1])
  return ARGV[2]
end
return 0
`)

var allowRequestScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
if count > tonumber(ARGV[2]) then
  return redis.call("TTL", KEYS[1])
end
return 0
`)

// Limiter implements login lockout and fixed-window gateway limiting.
type Limiter struct {
	client redis.UniversalClient
}

// NewLimiter constructs Redis-backed limiters.
func NewLimiter(client redis.UniversalClient) *Limiter {
	return &Limiter{client: client}
}

// Locked returns remaining lockout time, or zero when the client is not locked.
func (l *Limiter) Locked(ctx context.Context, clientKey string) (time.Duration, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return l.ttl(callCtx, loginLockPrefix, clientKey)
}

// RecordFailure increments failures and returns a lockout duration when triggered.
func (l *Limiter) RecordFailure(ctx context.Context, clientKey string, maxFailures int, lockout time.Duration) (time.Duration, error) {
	if maxFailures < 1 || lockout < time.Second {
		return 0, fmt.Errorf("invalid login limiter configuration")
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	result, err := recordFailureScript.Run(callCtx, l.client,
		[]string{hashedClientKey(loginFailurePrefix, clientKey), hashedClientKey(loginLockPrefix, clientKey)},
		maxFailures, int64(lockout/time.Second)).Int64()
	if err != nil {
		return 0, err
	}
	if result == 0 {
		return 0, nil
	}
	return time.Duration(result) * time.Second, nil
}

// Reset removes failed-attempt and lockout state after successful login.
func (l *Limiter) Reset(ctx context.Context, clientKey string) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return l.client.Del(callCtx,
		hashedClientKey(loginFailurePrefix, clientKey),
		hashedClientKey(loginLockPrefix, clientKey)).Err()
}

// Allow applies a fixed one-minute-style window with an explicit duration.
func (l *Limiter) Allow(ctx context.Context, clientKey string, limit int, window time.Duration) (time.Duration, error) {
	if limit < 1 || window < time.Second {
		return 0, fmt.Errorf("invalid rate limiter configuration")
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	seconds := int64(window / time.Second)
	result, err := allowRequestScript.Run(callCtx, l.client,
		[]string{hashedClientKey(ratePrefix, clientKey)}, seconds, limit).Int64()
	if err != nil {
		return 0, err
	}
	return time.Duration(result) * time.Second, nil
}

func (l *Limiter) ttl(ctx context.Context, prefix, clientKey string) (time.Duration, error) {
	seconds, err := l.client.TTL(ctx, hashedClientKey(prefix, clientKey)).Result()
	if err != nil || seconds <= 0 {
		return 0, err
	}
	return seconds * time.Second, nil
}
