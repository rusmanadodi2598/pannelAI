// Command app-serv wires the provider plugin dependencies.
//
// @file      cmd/app-serv/provider_wiring.go
// @for       Installs the connector registry and loads the embedded provider
//
//	registry at boot.
//
// @uses      internal/provider, internal/registry, fmt, log/slog.
// @reason    The provider seam is installed once, before the server accepts
//
//	traffic, so a provider patch or addition is a registration here
//	rather than a branch in shared code. Keeping it out of main.go
//	preserves the composition root's line budget, as auth_wiring.go
//	already does for authentication.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"fmt"
	"log/slog"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// buildProviderRuntime loads the embedded registry and installs the plugin
// registry, returning both for the layers that resolve provider ids.
//
// The two are distinct on purpose: the registry is data (which providers exist,
// and how to reach them) while the connectors are behaviour (the code that
// speaks a protocol the gateway does not translate itself). A provider needs a
// connector only when the format is not natively translated, which is why the
// unsupported list is logged here: it is the remaining work made visible, not a
// silent gap.
//
// Specialized connectors are registered by appending to the argument list below.
// Each is independent, so adding or fixing a provider touches one line there and
// nothing else.
func buildProviderRuntime() (*registry.Index, *provider.Connectors, error) {
	idx, err := registry.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("provider registry: %w", err)
	}

	// No specialized connector is registered yet. Providers on natively
	// translated formats are served by provider.Default, which is why the
	// registry is fully usable before any connector exists.
	connectors, err := provider.NewConnectors(provider.DefaultFactory)
	if err != nil {
		return nil, nil, fmt.Errorf("provider connectors: %w", err)
	}
	if err := provider.Register(connectors); err != nil {
		return nil, nil, fmt.Errorf("provider connectors: %w", err)
	}

	slog.Info("provider registry loaded",
		"revision", idx.Revision(),
		"providers", idx.Count(),
		"connectors", connectors.Count(),
	)

	// Logged rather than returned: a registry with providers the gateway cannot
	// serve yet is a working gateway with a smaller surface, not a boot failure.
	if unsupported := connectors.Unsupported(idx); len(unsupported) > 0 {
		slog.Warn("providers without a connector for their wire format",
			"count", len(unsupported),
			"providers", unsupported,
		)
	}
	return idx, connectors, nil
}
