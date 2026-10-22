// Package redis implements Redis-backed state repositories for app-serv.
//
// @file      internal/repository/redis/worker_lease.go
// @for       The SET NX lease and compare-and-delete release shared by the background workers.
// @uses      crypto/rand, encoding/hex, github.com/redis/go-redis/v9, context, time.
// @reason    A worker cycle must be single-owner across replicas, and a release must not delete a lease another replica has taken after this one expired; the Lua compare on the token is what makes the delete safe.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-22
package redisrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// WorkerLease is one process's claim on a worker cycle.
type WorkerLease struct {
	client redis.UniversalClient
	key    string
	token  string
	ttl    time.Duration
}

// NewWorkerLease builds a lease over a named worker. The token is generated once
// per process so a release can prove it still owns the key.
func NewWorkerLease(client redis.UniversalClient, name string, ttl time.Duration) (*WorkerLease, error) {
	if client == nil || name == "" {
		return nil, errors.New("redis: a worker lease needs a client and a name")
	}
	if ttl <= 0 {
		return nil, errors.New("redis: a worker lease needs a positive ttl")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &WorkerLease{
		client: client,
		key:    "pannelai:worker:" + name + ":lease",
		token:  hex.EncodeToString(raw),
		ttl:    ttl,
	}, nil
}

// Acquire takes the lease when no other process holds it.
func (l *WorkerLease) Acquire(ctx context.Context) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return l.client.SetNX(callCtx, l.key, l.token, l.ttl).Result()
}

// Release deletes the lease only while this process's token is still stored.
func (l *WorkerLease) Release(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	return workerLeaseReleaseScript.Run(callCtx, l.client, []string{l.key}, l.token).Err()
}

var workerLeaseReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)
