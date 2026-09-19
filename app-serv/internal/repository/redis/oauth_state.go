// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/oauth_state.go
// @for       The single-use OAuth state staging area behind §7.4's callback.
// @uses      github.com/redis/go-redis/v9, context, time.
// @reason    SPEC-API-001 §4 makes `state` a replay guard: one callback may
//
//	consume it, within ten minutes, exactly once. Staging with SET NX
//	refuses a guessed collision and GETDEL makes the take atomic, so two
//	concurrent callbacks with one state cannot both win — the loser is
//	told the state is gone, which is the replay answer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-19
package redisrepo

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

const oauthStateKeyPrefix = "pannelai:oauth:state:"

// OAuthStateStore stages OAuth states in Redis.
type OAuthStateStore struct {
	client redis.UniversalClient
}

// NewOAuthStateStore constructs a Redis-backed OAuth state store.
func NewOAuthStateStore(client redis.UniversalClient) *OAuthStateStore {
	return &OAuthStateStore{client: client}
}

// Stage records the state's payload once: SET NX refuses a value already
// staged, so a client retrying /oauth/start with a colliding state cannot
// silently replace the first flow's PKCE verifier.
func (s *OAuthStateStore) Stage(ctx context.Context, state string, payload []byte, ttl time.Duration) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	stored, err := s.client.SetNX(callCtx, oauthStateKey(state), payload, ttl).Result()
	if err != nil {
		return err
	}
	if !stored {
		return repository.ErrStateAlreadyStaged
	}
	return nil
}

// Take removes the state and returns its payload. A key that never existed,
// expired, or was taken before reports ok=false without an error, because a
// replay is the documented answer, not a storage failure.
func (s *OAuthStateStore) Take(ctx context.Context, state string) ([]byte, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	payload, err := s.client.GetDel(callCtx, oauthStateKey(state)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

func oauthStateKey(state string) string {
	return oauthStateKeyPrefix + state
}
