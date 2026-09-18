// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/endpoint_cursor.go
// @for       The round-robin cursor over one provider's upstream endpoints.
// @uses      github.com/redis/go-redis/v9, context, crypto/sha256, encoding/hex,
//
//	time.
//
// @reason    SPEC-API-001 §4 puts sticky round-robin state in Redis, and it has to
//
//	live there rather than in the process: a per-instance cursor makes
//	distribution depend on which replica answered, which is not
//	distribution at all. It is also the one piece the data plane's
//	`CursorStore` port has no implementation for, so this is that
//	implementation rather than a second copy of anything.
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
	"time"

	"github.com/redis/go-redis/v9"
)

// cursorKeyPrefix namespaces the cursor keys so they cannot collide with the
// limiter, session, or quota keys that share this Redis instance.
const cursorKeyPrefix = "pannelai:cursor:endpoint:"

// cursorTTL bounds the state so an abandoned provider's keys expire instead of
// accumulating forever. A day is far longer than any sticky budget, so it never
// interrupts live rotation.
const cursorTTL = 24 * time.Hour

// CursorStore advances the round-robin position over one provider's endpoints.
type CursorStore struct {
	client redis.UniversalClient
}

// NewCursorStore constructs a Redis-backed endpoint cursor.
func NewCursorStore(client redis.UniversalClient) *CursorStore {
	return &CursorStore{client: client}
}

// advanceScript moves the cursor and reports the offset to use.
//
// It runs as one script because the read-decide-write sequence must be atomic: two
// replicas selecting for the same provider at the same instant would otherwise
// both read the same offset and both stay on one account, which is exactly the
// failure round-robin exists to prevent.
//
// KEYS[1] offset key, KEYS[2] consecutive-use key.
// ARGV[1] pool size, ARGV[2] sticky limit, ARGV[3] ttl seconds.
// Returns the zero-based offset to start selecting at.
var advanceScript = redis.NewScript(`
local size = tonumber(ARGV[1])
if size <= 0 then
  return 0
end
local sticky = tonumber(ARGV[2])
if sticky < 1 then
  sticky = 1
end

local offset = tonumber(redis.call("GET", KEYS[1]) or "0")
-- A stored offset can exceed a pool that shrank, so it is reduced rather than
-- trusted: without this, deleting endpoints would strand the cursor past the end.
offset = offset % size

local used = tonumber(redis.call("GET", KEYS[2]) or "0")
if used < sticky then
  redis.call("SET", KEYS[2], used + 1, "EX", ARGV[3])
  return offset
end

local next = (offset + 1) % size
redis.call("SET", KEYS[1], next, "EX", ARGV[3])
redis.call("SET", KEYS[2], 1, "EX", ARGV[3])
return next
`)

// NextOffset returns the zero-based offset to start selecting at, then advances the
// cursor once the sticky budget is spent. A sticky budget keeps one account for a
// bounded number of calls, so one account serves a short burst instead of the pool
// alternating on every request.
func (s *CursorStore) NextOffset(ctx context.Context, providerID string, size int, stickyLimit int) (int, error) {
	if size <= 0 {
		return 0, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	key := cursorKeyPrefix + cursorProviderKey(providerID)
	offset, err := advanceScript.Run(callCtx, s.client,
		[]string{key, key + ":used"},
		size, stickyLimit, int64(cursorTTL/time.Second)).Int64()
	if err != nil {
		return 0, err
	}
	return int(offset), nil
}

// cursorProviderKey derives the key segment from a provider id.
//
// The id is hashed because it arrives from a request: a provider id containing a
// colon or a newline could otherwise fabricate a key that collides with another
// namespace, so the derived segment is fixed-width hex rather than anything the
// caller controls.
func cursorProviderKey(providerID string) string {
	digest := sha256.Sum256([]byte(providerID))
	return hex.EncodeToString(digest[:])
}
