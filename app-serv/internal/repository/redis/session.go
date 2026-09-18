// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/session.go
// @for       Stores revocable dashboard session digests with bounded TTLs.
// @uses      github.com/redis/go-redis/v9, context, crypto/sha256.
// @reason    Session cookies are opaque at the edge, while Redis provides
//
//	immediate revocation without keeping authentication state in Go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-17
package redisrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "pannelai:auth:session:"

const redisCallTimeout = 3 * time.Second

// SessionStore persists dashboard session digests in Redis.
type SessionStore struct {
	client redis.UniversalClient
}

// NewSessionStore constructs a Redis session store.
func NewSessionStore(client redis.UniversalClient) *SessionStore {
	return &SessionStore{client: client}
}

// Create records an active session for its configured lifetime.
func (s *SessionStore) Create(ctx context.Context, digest string, ttl time.Duration) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Set(callCtx, sessionKey(digest), "1", ttl).Err()
}

// Exists reports whether Redis still tracks the session digest.
func (s *SessionStore) Exists(ctx context.Context, digest string) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	count, err := s.client.Exists(callCtx, sessionKey(digest)).Result()
	return count == 1, err
}

// Revoke removes a session digest immediately.
func (s *SessionStore) Revoke(ctx context.Context, digest string) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Del(callCtx, sessionKey(digest)).Err()
}

func sessionKey(digest string) string {
	return sessionKeyPrefix + digest
}

func hashedClientKey(prefix, clientKey string) string {
	digest := sha256.Sum256([]byte(clientKey))
	return prefix + hex.EncodeToString(digest[:])
}
