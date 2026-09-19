// Command app-serv wires the §7.11 proxy pool graph.
//
// @file      cmd/app-serv/proxy_wiring.go
// @for       Builds the proxy prober adapter and the proxy service behind its
//
//	handler, over the process's egress guard.
//
// @uses      internal/config, internal/handler, internal/netguard,
//
//	internal/repository/postgres, internal/service, pgxpool.
//
// @reason    AGENTS.md §1.5 makes the composition root wiring only, and §1.1
//
//	keeps management_wiring.go inside its line budget. The guard is built
//	once in egress_wiring.go and passed in, because it is the process-wide
//	egress policy (OWASP A01): the proxy test is one caller among several,
//	and every one of them must share the same allowlist.
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
