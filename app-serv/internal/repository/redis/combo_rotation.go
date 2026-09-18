// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/combo_rotation.go
// @for       Advances the round-robin rotation state of a combo atomically.
// @uses      github.com/redis/go-redis/v9, internal/domain,
//
//	internal/repository, context, crypto/sha256, fmt, time.
//
// @reason    SPEC-API-001 §9.6 puts sticky round-robin state in Redis, and §7.7
//
//	makes the rotation a distribution rule: two concurrent requests for
//	one combo must land on different models rather than both reading
//	the same state. The script below therefore owns exactly one thing —
//	an atomic request counter — and the ordering rule stays in
//	domain.RotationRequestIndex, which a unit test runs without Redis.
//	Splitting it that way is what stops the stored state and the served
//	order from becoming two implementations of one rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package redisrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// comboRotationPrefix namespaces the rotation keys. The combo's name is hashed
// into the key rather than embedded, so a name an operator typed cannot collide
// with another key's namespace or inject a separator.
const comboRotationPrefix = "pannelai:combo:rotation:"

// comboRotationTTL bounds the state's lifetime. A combo that stops receiving
// traffic should not leave a key behind forever, and restarting its rotation
// costs one request of skew rather than a wrong answer.
const comboRotationTTL = 24 * time.Hour

// nextRotationScript increments the combo's request counter and returns the
// counter value the request that just arrived holds, zero-based. ARGV[1] is the
// TTL in seconds.
var nextRotationScript = redis.NewScript(`
local requests = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
return requests - 1
`)

// ComboRotationStore implements repository.ComboRotationStore on Redis.
type ComboRotationStore struct {
	client redis.UniversalClient
}

// NewComboRotationStore constructs the rotation store.
func NewComboRotationStore(client redis.UniversalClient) *ComboRotationStore {
	return &ComboRotationStore{client: client}
}

// Next returns the model order for this request.
//
// The script's counter is the only shared state; which model leads is
// domain.RotationRequestIndex applied to it, so the atomic step is a single
// INCR and the distribution rule has one implementation.
func (s *ComboRotationStore) Next(ctx context.Context, comboKey string, models []string, stickyLimit int) ([]string, error) {
	if len(models) < 2 {
		order := make([]string, len(models))
		copy(order, models)
		return order, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	requests, err := nextRotationScript.Run(callCtx, s.client,
		[]string{comboRotationKey(comboKey)},
		int64(comboRotationTTL/time.Second)).Int64()
	if err != nil {
		return nil, fmt.Errorf("advancing combo rotation: %w", err)
	}
	return domain.RotateRefs(models, domain.RotationRequestIndex(int(requests), stickyLimit, len(models))), nil
}

// Reset clears a combo's rotation state. A combo whose model list or strategy
// changed must not continue from a counter that addressed the old list.
func (s *ComboRotationStore) Reset(ctx context.Context, comboKey string) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Del(callCtx, comboRotationKey(comboKey)).Err()
}

// comboRotationKey derives the Redis key for a combo name.
func comboRotationKey(comboKey string) string {
	digest := sha256.Sum256([]byte(comboKey))
	return comboRotationPrefix + hex.EncodeToString(digest[:])
}
