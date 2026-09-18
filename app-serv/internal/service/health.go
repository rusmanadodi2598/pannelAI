// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/health.go
// @for       Dependency liveness for PostgreSQL and Redis (SPEC-API-001 §7.1).
// @uses      internal/domain, context, time.
// @reason    A health probe must distinguish "process up" from "dependencies
//
//	reachable" so an orchestrator does not route traffic to a server
//	that cannot serve. Dependencies arrive as Pinger interfaces, so
//	this layer never holds a driver (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-16
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// dependencyProbeTimeout bounds each liveness dependency check so a hung
// dependency degrades the report instead of stalling it (AGENTS.md §1.6).
//
// The name states what it bounds because this package also owns a connectivity
// test, whose budget is deliberately longer: a liveness probe must answer
// quickly while an operator waits, whereas a connectivity test is allowed to
// wait for a real upstream round trip.
const dependencyProbeTimeout = 3 * time.Second

// Pinger is the probe surface a dependency must offer. pgxpool.Pool satisfies
// it directly; a Redis client is adapted at the composition root.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthService reports the reachability of app-serv dependencies.
type HealthService struct {
	postgres Pinger
	redis    Pinger
}

// HealthServiceDeps holds the dependencies to probe. A nil entry is reported as
// skipped rather than failing the probe.
type HealthServiceDeps struct {
	Postgres Pinger
	Redis    Pinger
}

// NewHealthService returns a ready service.
func NewHealthService(deps HealthServiceDeps) *HealthService {
	return &HealthService{postgres: deps.Postgres, redis: deps.Redis}
}

// Check probes every configured dependency and returns the aggregate state plus
// one result per dependency.
//
// A failing probe reports only its state, never the driver's message: §7.1
// marks health as a public endpoint, and a connection error names the database
// and role it failed to reach. The full error goes to the log instead, where an
// operator can read it without exposing it to the internet.
func (s *HealthService) Check(ctx context.Context) (domain.HealthState, []domain.ProbeResult) {
	results := make([]domain.ProbeResult, 0, 2)

	postgres := s.probe(ctx, "postgres", s.postgres)
	redisResult := s.probe(ctx, "redis", s.redis)
	results = append(results, postgres, redisResult)

	if postgres.State == domain.ProbeDown || redisResult.State == domain.ProbeDown {
		return domain.HealthDown, results
	}
	return domain.HealthUP, results
}

// probe runs one dependency check under the probe timeout.
func (s *HealthService) probe(ctx context.Context, name string, p Pinger) domain.ProbeResult {
	if p == nil {
		return domain.ProbeResult{Name: name, State: domain.ProbeSkipped}
	}
	ctx, cancel := context.WithTimeout(ctx, dependencyProbeTimeout)
	defer cancel()

	if err := p.Ping(ctx); err != nil {
		slog.Warn("dependency probe failed", "dependency", name, "error", err)
		return domain.ProbeResult{Name: name, State: domain.ProbeDown}
	}
	return domain.ProbeResult{Name: name, State: domain.ProbeOK}
}
