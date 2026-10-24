// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/session.go
// @for       Stores revocable dashboard session digests with bounded TTLs.
// @uses      github.com/redis/go-redis/v9, context, crypto/sha256.
// @reason    Session cookies are opaque at the edge, while Redis provides immediate revocation without keeping authentication state in Go.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-17
package redisrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionKeyPrefix = "pannelai:auth:session:"
	sessionEpochKey  = "pannelai:auth:epoch"
)

const redisCallTimeout = 3 * time.Second

// SessionStore persists dashboard session digests in Redis.
type SessionStore struct {
	client redis.UniversalClient
}

// NewSessionStore constructs a Redis session store.
func NewSessionStore(client redis.UniversalClient) *SessionStore {
	return &SessionStore{client: client}
}

// Create records an active session for its configured lifetime at the epoch
// the login observed before reading the password hash.
func (s *SessionStore) Create(ctx context.Context, digest string, ttl time.Duration, epoch int64) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Set(callCtx, sessionKey(digest), strconv.FormatInt(epoch, 10), ttl).Err()
}

// Epoch returns the current credential epoch, or zero before the first rotation.
func (s *SessionStore) Epoch(ctx context.Context) (int64, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	epoch, err := s.client.Get(callCtx, sessionEpochKey).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return epoch, err
}

// Exists reports whether Redis tracks the digest at the current epoch. The two
// keys are read in one MGET so a rotation between the reads cannot pass.
func (s *SessionStore) Exists(ctx context.Context, digest string) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	values, err := s.client.MGet(callCtx, sessionKey(digest), sessionEpochKey).Result()
	if err != nil {
		return false, err
	}
	created, ok := values[0].(string)
	if !ok {
		return false, nil
	}
	current := "0"
	if values[1] != nil {
		if current, ok = values[1].(string); !ok {
			return false, fmt.Errorf("session epoch is %T, want string", values[1])
		}
	}
	return created == current, nil
}

// Revoke removes a session digest immediately.
func (s *SessionStore) Revoke(ctx context.Context, digest string) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Del(callCtx, sessionKey(digest)).Err()
}

// RevokeAll invalidates every session created before the call by advancing the
// credential epoch. One INCR replaces the scan-and-delete walk, so a password
// change signs out every client atomically; the old digests expire on their own
// TTL.
func (s *SessionStore) RevokeAll(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Incr(callCtx, sessionEpochKey).Err()
}

func sessionKey(digest string) string {
	return sessionKeyPrefix + digest
}

func hashedClientKey(prefix, clientKey string) string {
	digest := sha256.Sum256([]byte(clientKey))
	return prefix + hex.EncodeToString(digest[:])
}
