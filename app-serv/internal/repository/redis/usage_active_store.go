// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/usage_active_store.go
// @for       The in-flight marker set: one member per request being routed now,
//
//	scored by its start instant.
//
// @uses      github.com/redis/go-redis/v9, internal/domain, context, fmt, time.
// @reason    SPEC-UI-001 §6.5 makes the drawing's live state come from the
//
//	gateway, and the gateway is the only party that knows a request is
//	between two instants. The store lives in Redis rather than in
//	process memory because this service may run more than one replica:
//	an in-process map would give each replica its own idea of what is
//	active, and the stream would show a different set depending on which
//	replica answered.
//
//	A sorted set scored by the start instant is what makes the read
//	bounded: ZRANGEBYSCORE applies the staleness cutoff and the limit in
//	the server, so a read costs one command and never walks the
//	keyspace. The member is the marker's own encoded form, so removal is
//	one exact ZREM with no lookup, and the prune is one
//	ZREMRANGEBYSCORE over the same key: there is no second key to keep in
//	step and therefore no orphaned entry to leak.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-22
package redisrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// activeRequestKey is the single sorted set holding every in-flight marker.
// One key is correct because the set is global: the drawing shows one gateway.
const activeRequestKey = "pannelai:usage:active"

// activeRequestCallTimeout bounds every Redis round trip so a hung server
// degrades the live read instead of stalling the call it is describing
// (AGENTS.md §1.6).
const activeRequestCallTimeout = 3 * time.Second

// ActiveRequestStore is the Redis implementation of
// repository.ActiveRequestStore.
type ActiveRequestStore struct {
	client redis.UniversalClient
}

// NewActiveRequestStore constructs the store over an existing client.
func NewActiveRequestStore(client redis.UniversalClient) *ActiveRequestStore {
	return &ActiveRequestStore{client: client}
}

// Start records one marker. The encode validates first, so a marker no reader
// would draw never reaches the set.
func (s *ActiveRequestStore) Start(ctx context.Context, marker domain.ActiveRequest) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload, err := domain.EncodeActiveRequest(marker)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, activeRequestCallTimeout)
	defer cancel()
	if err := s.client.ZAdd(callCtx, activeRequestKey, redis.Z{
		Score:  marker.Score(),
		Member: payload,
	}).Err(); err != nil {
		return fmt.Errorf("recording active request: %w", err)
	}
	return nil
}

// Finish removes one marker. The member is the marker's own encoding, so this is
// one exact deletion: a marker already removed by the staleness prune or by a
// restart reports zero removed, which is the state the caller asked for.
func (s *ActiveRequestStore) Finish(ctx context.Context, marker domain.ActiveRequest) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload, err := domain.EncodeActiveRequest(marker)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, activeRequestCallTimeout)
	defer cancel()
	if err := s.client.ZRem(callCtx, activeRequestKey, payload).Err(); err != nil {
		return fmt.Errorf("removing active request: %w", err)
	}
	return nil
}

// Active returns the live markers, oldest first, bounded by limit.
//
// The stale members are pruned before the read rather than after it, so the set
// cannot accumulate entries from a process that died mid-request: the prune and
// the read are two commands under one deadline, and a read that fails leaves the
// set no larger than it was.
//
// A member that fails to decode is removed and skipped rather than failing the
// read: the value came from a shared Redis instance, so one foreign member must
// not be able to blank the whole drawing (OWASP A08).
func (s *ActiveRequestStore) Active(ctx context.Context, now time.Time, limit int) ([]domain.ActiveRequest, error) {
	if s == nil || s.client == nil || limit < 1 {
		return nil, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, activeRequestCallTimeout)
	defer cancel()

	cutoff := domain.ActiveRequestCutoff(now)
	// reason: the prune is best effort. A failure to remove stale members must
	// not fail the read that follows it, because the read applies the same
	// cutoff and the next attempt prunes again; reporting the prune as the
	// error would turn a stale-marker cleanup into a blank drawing.
	_ = s.client.ZRemRangeByScore(callCtx, activeRequestKey,
		"-inf", fmt.Sprintf("(%d", cutoff.UnixMilli())).Err()

	raw, err := s.client.ZRangeByScore(callCtx, activeRequestKey, &redis.ZRangeBy{
		Min:    fmt.Sprintf("(%d", cutoff.UnixMilli()),
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("reading active requests: %w", err)
	}

	markers := make([]domain.ActiveRequest, 0, len(raw))
	for _, member := range raw {
		marker, decodeErr := domain.DecodeActiveRequest([]byte(member))
		if decodeErr != nil {
			// A member this build cannot read is one nothing here wrote, so it
			// is dropped and counted against the keyspace rather than the read.
			s.discard(callCtx, member)
			continue
		}
		markers = append(markers, marker)
	}
	return markers, nil
}

// discard removes one unreadable member, so a foreign value cannot be returned
// on every later read.
func (s *ActiveRequestStore) discard(ctx context.Context, member string) {
	// reason: discarding a foreign member is cleanup, and the read it belongs to
	// has already answered; a failed removal retries on the next read.
	_ = s.client.ZRem(ctx, activeRequestKey, member).Err()
}
