// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/registry_surface.go
// @for       The registry questions the resolver and the models list ask.
// @uses      internal/registry.
// @reason    The gateway reads providers through a surface rather than the
//
//	concrete index so a node the operator created after boot is routable by
//	the next request. The interface lives apart from resolve.go because the
//	resolution rules and the surface they are asked through are two concerns,
//	and one file carrying both had reached the AGENTS.md §1.1 ceiling.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"

// ProviderRegistry is the registry surface the resolver needs: one provider
// lookup by id, alias, or node prefix, one declared-model lookup inside it, and
// the full list the models endpoint enumerates.
//
// It is an interface rather than *registry.Index because the composition root
// overlays the stored custom nodes on the embedded registry. A node created
// through POST /provider-nodes must be routable by the next request, and a
// boot-frozen index would accept the create and then refuse every request aimed
// at it, which reads as a routing bug rather than as a stale registry.
type ProviderRegistry interface {
	// Provider resolves an id, alias, or node prefix to its entry.
	Provider(name string) (registry.Provider, bool)
	// Model resolves a declared model inside a provider.
	Model(providerName, modelID string) (registry.Model, bool)
	// All returns every entry, embedded and custom.
	All() []registry.Provider
}
