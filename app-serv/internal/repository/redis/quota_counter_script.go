// Package redisrepo implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/quota_counter_script.go
// @for       The two atomic scripts behind the quota counter: the rollover
//
//	advance and the post-flush settle.
//
// @uses      github.com/redis/go-redis/v9.
// @reason    Both scripts compare a stored instant against the server's clock,
//
//	which only Redis can do atomically with the counter read. A
//	read-then-write from Go would let a flush or a request land between
//	the two and see a half-rolled window (AGENTS.md §2.1: the race is
//	real, not theoretical, at a 30s tick against live traffic).
//
//	Instants travel as unix MICROseconds, not nanoseconds: Lua numbers
//	are doubles, and a nanosecond instant (~1.7e18) is past the 2^53
//	integer range where a comparison starts rounding. Microseconds
//	(~1.7e15) compare exactly.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-23
package redisrepo

import "github.com/redis/go-redis/v9"

// addScript advances one window's running total, restarting the window when its
// reset instant has passed. The rollover, the reset write, and the increment are
// one script so no reader can observe the total restarted but the old reset
// still stored, or the increment landed in a window that had already closed.
//
// KEYS[1] is the endpoint's hash. ARGV holds the counter field, the reset field,
// the flushed field, the units, and the new reset instant as unix microseconds.
//
// A restart drops the flushed marker: the new window's total is not the one the
// last flush mirrored, even when the two happen to hold the same number.
//
// The stored reset is compared numerically, so an instant an older deployment
// wrote as RFC 3339 text (unparseable as a number) reads as closed: the window
// restarts once at the upgrade and is numeric from then on.
var addScript = redis.NewScript(`
local clock = redis.call("TIME")
local now = clock[1] * 1000000 + clock[2]
local stored = redis.call("HGET", KEYS[1], ARGV[2])
local restart = stored == false
if not restart then
  local closed = tonumber(stored)
  if closed == nil or now >= closed then
    restart = true
  end
end
if restart then
  redis.call("HDEL", KEYS[1], ARGV[1], ARGV[3])
  redis.call("HSET", KEYS[1], ARGV[2], ARGV[5])
end
return redis.call("HINCRBY", KEYS[1], ARGV[1], ARGV[4])
`)

// settleScript runs after a durable write. For each window it marks the flushed
// total, and retires the hot copy when the window has closed and that total is
// still the one stored: nothing would read it again, and the next request
// restarts the window anyway.
//
// A window whose total changed since the flush keeps its counter, because those
// units are not durable anywhere yet. An open window keeps it too: that total is
// the running number the next flush mirrors.
//
// KEYS[1] is the endpoint's hash. ARGV[1] is the flushed-field prefix, ARGV[2]
// the window count, then that many triples of counter field, reset field, and
// the used value the flush wrote.
var settleScript = redis.NewScript(`
local clock = redis.call("TIME")
local now = clock[1] * 1000000 + clock[2]
local retired = 0
for i = 0, tonumber(ARGV[2]) - 1 do
  local base = 3 + i * 3
  local counter = redis.call("HGET", KEYS[1], ARGV[base])
  if counter ~= false then
    local stored = redis.call("HGET", KEYS[1], ARGV[base + 1])
    local closed = tonumber(stored)
    if counter == ARGV[base + 2] and (closed == nil or now >= closed) then
      redis.call("HDEL", KEYS[1], ARGV[base], ARGV[base + 1], ARGV[1] .. ARGV[base])
      retired = retired + 1
    else
      redis.call("HSET", KEYS[1], ARGV[1] .. ARGV[base], ARGV[base + 2])
    end
  end
end
return retired
`)
