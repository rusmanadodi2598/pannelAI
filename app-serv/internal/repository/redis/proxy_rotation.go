// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/proxy_rotation.go
// @for       The proxy route engine's state: the round-robin counter and the
//
//	connect-failure cooldown (docs/PORT/008-PORT-PROXY-ENGINE.md D5, D6).
//
// @uses      github.com/redis/go-redis/v9, context, crypto/sha256, encoding/hex,
//
//	fmt, time.
//
// @reason    D5 makes the rotation cursor atomic across concurrent requests and
//
//	durable across restarts, which is exactly what the combo rotation's
//	one INCR script already is; reusing the shape keeps the two rotation
//	rules identical where they overlap. The cooldown is a SET with a TTL
//	because a parked candidate must return to service without anyone
//	remembering to unpark it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-26
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

// proxyRotationPrefix namespaces the round-robin counter. The pool key is
// hashed into it rather than embedded, mirroring the combo store, so a key an
// operator-typed name could not collide with stays a non-question.
const proxyRotationPrefix = "pannelai:proxy:rotation:"

// proxyParkPrefix namespaces the failure-cooldown keys.
const proxyParkPrefix = "pannelai:proxy:park:"

// proxyRotationTTL bounds the counter's lifetime (D5): a pool that stops
// receiving traffic leaves no key behind forever, and restarting the rotation
// costs one request of skew rather than a wrong answer.
const proxyRotationTTL = 24 * time.Hour

// nextProxyRotationScript is the combo rotation's one atomic step pointed at
// the proxy namespace: increment, refresh the lifetime, answer zero-based.
var nextProxyRotationScript = redis.NewScript(`
local requests = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
return requests - 1
`)

// ProxyRouteStore implements repository.ProxyRouteStore on Redis.
type ProxyRouteStore struct {
	client redis.UniversalClient
}

// NewProxyRouteStore constructs the store.
func NewProxyRouteStore(client redis.UniversalClient) *ProxyRouteStore {
	return &ProxyRouteStore{client: client}
}

// Next returns the order this request uses (D5): the counter advances by one
// atomic step and domain.RotateRefs applies what the count means, the same
// split the combo rotation pins, so the stored state and the served order
// cannot disagree. A pool of fewer than two candidates never touches Redis.
func (s *ProxyRouteStore) Next(ctx context.Context, poolKey string, ids []string) ([]string, error) {
	if len(ids) < 2 {
		order := make([]string, len(ids))
		copy(order, ids)
		return order, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	requests, err := nextProxyRotationScript.Run(callCtx, s.client,
		[]string{proxyRotationKey(poolKey)},
		int64(proxyRotationTTL/time.Second)).Int64()
	if err != nil {
		return nil, fmt.Errorf("advancing proxy rotation: %w", err)
	}
	return domain.RotateRefs(ids, domain.RotationRequestIndex(int(requests), 1, len(ids))), nil
}

// Park records a connect-stage failure for ttl (D6). The TTL is the cooldown:
// the candidate returns to service when it expires, with no sweeper to run and
// no operator action to remember.
func (s *ProxyRouteStore) Park(ctx context.Context, proxyID string, ttl time.Duration) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Set(callCtx, proxyParkKey(proxyID), "1", ttl).Err()
}

// Parked reports whether the candidate is inside its cooldown (D6).
func (s *ProxyRouteStore) Parked(ctx context.Context, proxyID string) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	count, err := s.client.Exists(callCtx, proxyParkKey(proxyID)).Result()
	if err != nil {
		return false, fmt.Errorf("reading proxy cooldown: %w", err)
	}
	return count > 0, nil
}

// proxyRotationKey derives the counter's key for one pool key.
func proxyRotationKey(poolKey string) string {
	digest := sha256.Sum256([]byte(poolKey))
	return proxyRotationPrefix + hex.EncodeToString(digest[:])
}

// proxyParkKey derives the cooldown key for one candidate id.
func proxyParkKey(proxyID string) string {
	digest := sha256.Sum256([]byte(proxyID))
	return proxyParkPrefix + hex.EncodeToString(digest[:])
}
