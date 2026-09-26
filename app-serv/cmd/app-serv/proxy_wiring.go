// Command app-serv wires the §7.11 proxy pool graph.
//
// @file      cmd/app-serv/proxy_wiring.go
// @for       Builds the proxy prober adapter, the proxy service behind its
//
//	handler, and the pool-driven route planner the data plane dials
//	through, over the process's egress guard.
//
// @uses      internal/config, internal/dataplane, internal/domain, internal/handler,
//
//	internal/netguard, internal/repository/postgres, internal/repository/redis,
//	internal/service, context, pgxpool, redis.
//
// @reason    AGENTS.md §1.5 makes the composition root wiring only, and §1.1
//
//	keeps management_wiring.go inside its line budget. The guard is built
//	once in egress_wiring.go and passed in, because it is the process-wide
//	egress policy (OWASP A01): the proxy test is one caller among several,
//	and every one of them must share the same allowlist. The route
//	planner (docs/PORT/008-PORT-PROXY-ENGINE.md D8) is wired beside the
//	handler because both serve the pool: a plan that walks candidates
//	must draw on the same rows the operator edits, and a proxied request
//	must not switch the destination check off: the planner owns that
//	check because its per-attempt transports dial the proxy, not the
//	destination.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildProxies assembles the proxy pool handler over the shared egress guard.
func buildProxies(cfg config.Config, pool *pgxpool.Pool, sealer service.CredentialSealer, guard *netguard.Guard) (*handler.ProxyHandler, error) {
	svc, err := service.NewProxyService(service.ProxyServiceDeps{
		Repo:   postgres.NewProxyRepository(pool),
		Sealer: sealer,
		Prober: newProxyProber(guard, cfg.ProxyTestURL),
	})
	if err != nil {
		return nil, fmt.Errorf("proxy wiring: %w", err)
	}
	return handler.NewProxyHandler(svc), nil
}

// buildProxyRoutePlanner assembles the pool-driven route planner the chat and
// media transports walk (docs/PORT/008-PORT-PROXY-ENGINE.md D8): the route
// service's plan wrapped by one destination check per request. It cannot fail:
// the service validates nothing at construction, and the guard arrives built.
func buildProxyRoutePlanner(pool *pgxpool.Pool, rdb redis.UniversalClient, settings networkSettingsReader, sealer service.CredentialSealer, guard *netguard.Guard) dataplane.ProxyRoutePlanner {
	routes := service.NewProxyRouteService(service.ProxyRouteDeps{
		Proxies:  postgres.NewProxyRepository(pool),
		Routes:   redisrepo.NewProxyRouteStore(rdb),
		Settings: settings,
		Opener:   sealer,
	})
	return guardedProxyPlanner{routes: routes, guard: guard}
}

// guardedProxyPlanner satisfies the data plane's planner port. The destination
// check runs once per planned request, and only when the plan routes through a
// proxy: an empty plan serves the request on the shared client, whose dialer
// still validates the destination at connect time, while a proxied request
// never dials the destination, so without this check enabling the pool would
// switch the A01 policy off for every call it routes.
type guardedProxyPlanner struct {
	routes *service.ProxyRouteService
	guard  *netguard.Guard
}

// Plan returns the proxy attempts one request walks, after validating the
// destination the plan would hide behind the proxy.
func (p guardedProxyPlanner) Plan(ctx context.Context, host string) ([]domain.ProxyRouteAttempt, error) {
	plan, err := p.routes.Plan(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(plan) == 0 {
		return plan, nil
	}
	if err := p.guard.CheckHost(ctx, host); err != nil {
		return nil, err
	}
	return plan, nil
}

// ReportFailure parks one candidate for the cooldown, through to the service
// that owns the rule.
func (p guardedProxyPlanner) ReportFailure(ctx context.Context, proxyID string) {
	p.routes.ReportFailure(ctx, proxyID)
}
