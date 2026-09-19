// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/vision_rotation.go
// @for       Persists the vision adapter's round-robin position under one key.
// @uses      github.com/redis/go-redis/v9, internal/domain, context,
//
//	crypto/sha256, encoding/json, encoding/hex, time.
//
// @reason    SPEC-API-001 §7.8 asks the adapter to respect round_robin, which
//
//	needs the rotation state to outlive the request that advanced it.
//	The state is advisory — a lost key costs one request of skew, not
//	a wrong answer — so the store is a plain read and write under a
//	namespaced, hashed key with a TTL, rather than the scripted
//	atomic counter the combo rotation needs to be a distribution rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package redisrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// visionRotationPrefix namespaces the adapter's rotation key. One adapter
// exists per deployment, so the key needs no further discriminator; the hash
// keeps the namespace free of anything an operator typed.
const visionRotationPrefix = "pannelai:vision:rotation:"

// visionRotationTTL bounds the state's lifetime, for the same reason the combo
// rotation carries one: a configuration that stops receiving traffic should
// not leave a key behind forever.
const visionRotationTTL = 24 * time.Hour

// VisionRotationStore implements repository.VisionRotationStore on Redis.
type VisionRotationStore struct {
	client redis.UniversalClient
}

// NewVisionRotationStore constructs the rotation store.
func NewVisionRotationStore(client redis.UniversalClient) *VisionRotationStore {
	return &VisionRotationStore{client: client}
}

// Get returns the stored rotation state, or the zero state when none is stored
// or the stored value no longer decodes. A stale shape is treated as "start
// over" rather than as a failure: the next Save rewrites it.
func (s *VisionRotationStore) Get(ctx context.Context) (domain.RotationState, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()

	raw, err := s.client.Get(callCtx, visionRotationKey()).Bytes()
	if err != nil {
		return domain.RotationState{}, fmt.Errorf("reading vision rotation: %w", err)
	}
	var state domain.RotationState
	if err := json.Unmarshal(raw, &state); err != nil {
		return domain.RotationState{}, fmt.Errorf("decoding vision rotation: %w", err)
	}
	return state, nil
}

// Save replaces the stored rotation state and restarts its TTL.
func (s *VisionRotationStore) Save(ctx context.Context, state domain.RotationState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encoding vision rotation: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return s.client.Set(callCtx, visionRotationKey(), encoded, visionRotationTTL).Err()
}

// visionRotationKey derives the Redis key the adapter's rotation occupies.
func visionRotationKey() string {
	digest := sha256.Sum256([]byte(visionRotationPrefix))
	return visionRotationPrefix + hex.EncodeToString(digest[:])
}
