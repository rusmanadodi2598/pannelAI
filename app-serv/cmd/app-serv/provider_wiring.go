// Command app-serv wires the provider plugin dependencies.
//
// @file      cmd/app-serv/provider_wiring.go
// @for       Installs the connector registry and loads the embedded provider registry at boot.
// @uses      internal/provider, internal/registry, fmt, log/slog.
// @reason    The provider seam is installed once, before the server accepts traffic, so a provider patch or addition is a registration here rather than a branch in shared code. Keeping it out of main.go preserves the composition root's line budget, as auth_wiring.go already does for authentication.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-17
package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// buildProviderRuntime loads the embedded registry and installs the plugin
// registry, returning both for the layers that resolve provider ids. The registry
// is data, the connectors behaviour: a provider needs a connector only when its
// format is not natively translated, and one still missing is logged, not fatal.
// egressClient is the guarded client the egress policy built, so a connector's
// own outbound calls, the Qoder Personal Access Token exchange above all, ride
// the same allowlist as every other upstream call. That is why the egress
// foundation is wired before this in the boot sequence.
func buildProviderRuntime(egressClient *http.Client) (*registry.Index, *provider.Connectors, error) {
	idx, err := registry.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("provider registry: %w", err)
	}

	// Specialized connectors are registered here: a provider whose outbound
	// request must be rewritten, or that only answers a stream, cannot be served
	// by provider.Default. Each entry is independent, so a provider stays
	// patchable in isolation. The OpenCode lanes (free, PAYG, subscription) share
	// one connector because they share one upstream protocol, differing only in
	// credential and endpoint table. Each is registered by name and the connector
	// reads the entry it was built for, so a new lane is one line here.
	plugins := make([]provider.Plugin, 0, 7)
	for _, id := range []string{"opencode", "opencode-zen", "opencode-go"} {
		entry, ok := idx.Provider(id)
		if !ok {
			return nil, nil, fmt.Errorf("provider connectors: the %s entry is missing from the registry", id)
		}
		plugins = append(plugins, provider.NewOpenCode(entry))
	}
	// The Qoder pair shares one connector and differs only in the registry entry it
	// reads, the same way the OpenCode lanes do: the CN site declares one gateway for
	// every token kind. The guarded egress client is passed so the Personal Access
	// Token exchange, the catalog, and the identity reads all ride the process
	// egress guard.
	for _, id := range []string{"qoder", "qoder-cn"} {
		entry, ok := idx.Provider(id)
		if !ok {
			return nil, nil, fmt.Errorf("provider connectors: the %s entry is missing from the registry", id)
		}
		connector, err := provider.NewQoder(entry, egressClient)
		if err != nil {
			return nil, nil, fmt.Errorf("provider connectors: %w", err)
		}
		plugins = append(plugins, connector)
	}
	// CodeBuddy needs a connector for two things: the reference forces every request on
	// this service to a stream, and the vendor answers a plain OpenAI message list with
	// `11101 invalid request`, so the body has to be rebuilt before it leaves. Forced
	// streaming is declared by a connector here rather than read from the registry entry.
	// Everything else about the provider, URL, headers, bearer credential, is the plain
	// OpenAI wire the fallback already serves.
	for _, id := range []string{"codebuddy-cn", "codebuddy-intl"} {
		entry, ok := idx.Provider(id)
		if !ok {
			return nil, nil, fmt.Errorf("provider connectors: the %s entry is missing from the registry", id)
		}
		plugins = append(plugins, provider.NewCodeBuddy(entry))
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, plugins...)
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
