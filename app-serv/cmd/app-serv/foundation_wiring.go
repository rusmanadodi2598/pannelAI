// Command app-serv wires the process-wide collaborators the graph is built on.
//
// @file      cmd/app-serv/foundation_wiring.go
// @for       Builds the settings service, the egress policy, and the credential
//
//	sealer: the three collaborators every other builder reads.
//
// @uses      internal/config, internal/domain, internal/repository/postgres,
//
//	internal/service, fmt, pgxpool.
//
// @reason    These three are built before anything else and read by everything
//
//	else, so giving them their own builder keeps buildManagement's
//	sequence about the graph rather than about its foundation, and keeps
//	both files inside the AGENTS.md §1.1 budget.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildFoundation builds the collaborators every other builder depends on: the
// settings service (the egress route and the log service read it), the
// process-wide egress policy, and the credential sealer.
//
// The sealer is created once and shared: sealing and opening must agree on the
// key, so a second instance would be a second source of truth for it.
func buildFoundation(cfg config.Config, pool *pgxpool.Pool) (*service.SettingsService, egress, *domain.Sealer, error) {
	settingsRepo := postgres.NewSettingsRepository(pool)
	settingsSvc, err := service.NewSettingsService(service.SettingsServiceDeps{
		Repo: settingsRepo,
		// The per-provider binding (docs/PORT/009-PORT-PROVIDER-PROXY.md D6)
		// names a stored pool row, so the settings write needs the same one
		// question the endpoint parity check asks. It is built from the same
		// pool, so both services refuse a dangling id identically.
		Proxies: proxyPoolFinder{proxies: postgres.NewProxyRepository(pool)},
	})
	if err != nil {
		return nil, egress{}, nil, fmt.Errorf("management wiring: settings: %w", err)
	}

	// The process-wide egress policy (OWASP A01): one guard and one guarded HTTP
	// client, shared by the probe, the data plane, the OAuth client, and the
	// proxy test. Built once so a second caller cannot grow a second allowlist.
	policy, err := buildEgress(cfg, settingsSvc)
	if err != nil {
		return nil, egress{}, nil, fmt.Errorf("management wiring: %w", err)
	}

	sealer, err := domain.NewSealer([]byte(cfg.EncryptionKey))
	if err != nil {
		return nil, egress{}, nil, fmt.Errorf("management wiring: %w", err)
	}
	return settingsSvc, policy, sealer, nil
}
