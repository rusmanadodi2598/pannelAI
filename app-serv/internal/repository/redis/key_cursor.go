// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/key_cursor.go
// @for       The round-robin counter over one endpoint's keys, the atomic arbiter that spreads a burst across a credential set.
// @uses      github.com/redis/go-redis/v9, context, crypto/sha256, encoding/hex, time.
// @reason    Key selection used to be derived from the last time a key SUCCEEDED, so N concurrent requests read one identical timestamp and all chose the same credential, and a key the network refused was never stamped at all and won every later choice forever. A counter handed out atomically at selection time is the only thing that separates the two requests, and it lives here rather than in PostgreSQL because the hot path must not gain a row lock to spread load.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-14
package redisrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

// keyCursorPrefix namespaces the per-endpoint counters, beside the endpoint cursor
// rather than inside its namespace: one counts accounts of a provider, the other
// credentials of one account, and mixing them would advance one when it meant the other.
const keyCursorPrefix = "pannelai:cursor:key:"

// keyCursorTTL bounds the counter so an account whose keys were all removed does not
// leave a key behind forever; a day outlasts any rotation cycle the gateway runs.
const keyCursorTTL = 24 * time.Hour

// advanceCounter hands out the next raw count and keeps the key alive. INCR is the whole
// arbiter: two selectors asking at the same instant get different numbers, which is what
// ordering by the last success could not give. The value stays content-free; the caller
// takes the modulo, because the divisor is how many keys routing may spend now.
var advanceCounter = redis.NewScript(`
local next = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
return next
`)

// KeyCursorStore advances the round-robin position over one endpoint's keys.
type KeyCursorStore struct {
	client redis.UniversalClient
}

// NewKeyCursorStore constructs a Redis-backed key cursor.
func NewKeyCursorStore(client redis.UniversalClient) *KeyCursorStore {
	return &KeyCursorStore{client: client}
}

// Next returns the next raw count for one endpoint. The number is only meaningful
// modulo the caller's current key count, and it only ever grows.
func (s *KeyCursorStore) Next(ctx context.Context, endpointID string) (int, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	next, err := advanceCounter.Run(callCtx, s.client,
		[]string{keyCursorPrefix + cursorSegment(endpointID)}, int64(keyCursorTTL/time.Second)).Int()
	if err != nil {
		return 0, err
	}
	return next, nil
}

// cursorSegment derives the key segment from an id that arrives from a request, so
// an id holding a colon or a newline cannot fabricate a segment that collides with
// another namespace. The same hashing the endpoint cursor uses, for the same reason.
func cursorSegment(id string) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:])
}
