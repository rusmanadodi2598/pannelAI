// Command app-serv wires the §7.11 proxy pool graph.
//
// @file      cmd/app-serv/proxy_wiring.go
// @for       Builds the egress guard, the proxy prober adapter, and the proxy
//
//	service behind its handler.
//
// @uses      internal/config, internal/handler, internal/netguard,
//
//	internal/repository/postgres, internal/service, pgxpool.
//
// @reason    AGENTS.md §1.5 makes the composition root wiring only, and §1.1
//
//	keeps management_wiring.go inside its line budget. The guard is built
//	here because it is the process-wide egress policy (OWASP A01): the
//	proxy test is the first caller, and a second caller must share this
//	instance rather than grow a second allowlist.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildProxies assembles the proxy pool handler over the shared egress guard.
func buildProxies(cfg config.Config, pool *pgxpool.Pool, sealer service.CredentialSealer) (*handler.ProxyHandler, error) {
	guard, err := netguard.NewGuard(cfg.EgressAllowedTargets)
	if err != nil {
		return nil, fmt.Errorf("proxy wiring: egress guard: %w", err)
	}
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
