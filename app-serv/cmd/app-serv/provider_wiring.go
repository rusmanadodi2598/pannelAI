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
	"net/http"

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
// egressClient is the guarded client the egress policy built (foundation
// wiring), so a connector's own outbound calls — the Qoder Personal Access
// Token exchange above all — ride the same allowlist every other upstream call
// does (draft 042 R07). It is why the foundation is built before this in the
// boot sequence.
//
// Specialized connectors are registered by appending to the argument list below.
// Each is independent, so adding or fixing a provider touches one line there and
// nothing else.
func buildProviderRuntime(egressClient *http.Client) (*registry.Index, *provider.Connectors, error) {
	idx, err := registry.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("provider registry: %w", err)
	}

	// Specialized connectors are registered here. A provider that needs its
	// outbound request rewritten, or that only answers a stream, cannot be served
	// by provider.Default, so it needs an entry in this list. Each is independent,
	// which is what makes a provider patchable in isolation.
	// The OpenCode family shares one connector because it shares one upstream
	// protocol: the free lane, the PAYG lane, and the subscription lane differ in
	// credential and endpoint table, not in how a request is shaped. Each entry is
	// registered by name, and the connector reads the entry it was built for, so a
	// lane added to the registry is one line here.
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
	// egress guard (draft 042 R07).
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
	// Everything else about the provider — URL, headers, bearer credential — is the plain
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
