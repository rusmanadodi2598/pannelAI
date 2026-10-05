// Package repository is the storage boundary app-serv services depend on.
//
// @file      internal/repository/proxy_route.go
// @for       State contract for the proxy route engine: the round-robin
//
//	counter and the failure cooldown (docs/PORT/008-PORT-PROXY-ENGINE.md
//	D5, D6).
//
// @uses      context, time.
// @reason    The engine's shared state is distribution and health hints, both
//
//	of which must be atomic across concurrent requests and survive a
//	restart, which is Redis's job and not the process's. The contract
//	lives apart from ProxyRepository because the route plan reads pool
//	rows but writes none of them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-09-26
package repository

import (
	"context"
	"time"
)

// ProxyRouteStore is the Redis-backed state the proxy route engine reads and
// writes (docs/PORT/008-PORT-PROXY-ENGINE.md D5, D6).
type ProxyRouteStore interface {
	// Next advances the round-robin counter for the pool and returns the ids
	// rotated so the entry at the counter's index leads. A store that cannot
	// answer returns an error, which the caller treats as "keep the given
	// order": rotation is an optimisation, not a correctness input.
	Next(ctx context.Context, poolKey string, ids []string) ([]string, error)

	// Park records a connect-stage failure against one candidate for ttl. A
	// parked candidate is skipped while an unparked one remains.
	Park(ctx context.Context, proxyID string, ttl time.Duration) error

	// ParkedAll reports which candidates are inside their failure cooldown, for a
	// whole set at once. A proxy plan is built on every proxied request, so asking
	// per candidate is the query-in-a-loop shape AGENTS.md §1.7 blocks; the
	// single-candidate read went with the last caller that needed it.
	ParkedAll(ctx context.Context, proxyIDs []string) (map[string]bool, error)
}
