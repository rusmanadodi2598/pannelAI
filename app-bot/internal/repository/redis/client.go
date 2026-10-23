// Package redis owns the Redis client.
//
// @file      internal/repository/redis/client.go
// @for       Building the bounded Redis client used by the queue and locks.
// @uses      context, fmt, time, redis.
// @reason    The queue, the duplicate guard, and the rate limiter share one client with explicit timeouts, so a Redis stall cannot hold an HTTP request open indefinitely. The wrapper exists because go-redis returns a command, not an error, and the readiness probe wants an error.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-21
package redis

import (
	"context"
	"fmt"
	"time"

	redisc "github.com/redis/go-redis/v9"
)

const (
	dialTimeout = 5 * time.Second
	ioTimeout   = 3 * time.Second
	poolSize    = 10
	minIdle     = 2
	pingTimeout = 5 * time.Second
)

// Client wraps go-redis so the rest of the service depends on an error-returning
// Ping, which is the shape the readiness probe uses.
type Client struct {
	inner *redisc.Client
}

// NewClient builds the client and verifies the connection before returning.
func NewClient(ctx context.Context, addr, password string, db int) (*Client, error) {
	client := redisc.NewClient(&redisc.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  dialTimeout,
		ReadTimeout:  ioTimeout,
		WriteTimeout: ioTimeout,
		PoolSize:     poolSize,
		MinIdleConns: minIdle,
	})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("reaching Redis: %w", err)
	}
	return &Client{inner: client}, nil
}

// Ping answers nil when the server responds.
func (c *Client) Ping(ctx context.Context) error {
	return c.inner.Ping(ctx).Err()
}

// Close releases the pool.
func (c *Client) Close() error {
	return c.inner.Close()
}
