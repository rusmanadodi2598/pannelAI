// Command app-serv adapts the provider registry to the service's runtime lookup
//
// @file      cmd/app-serv/provider_index.go
// @for       The runtime ProviderIndex: embedded registry entries overlaid with
//
//	the custom nodes stored in PostgreSQL.
//
// @uses      context, sync, internal/domain, internal/registry, internal/service.
// @reason    service.ProviderIndex is deliberately an interface rather than a
//
//	*registry.Index because a node created through POST /provider-nodes
//	must become usable as a provider_id immediately. A boot-frozen index
//	would accept the create and then refuse every endpoint aimed at it,
//	which reads as a bug rather than as a limitation. This adapter is
//	the composition root's job: it is the only layer allowed to know
//	both the embedded registry and the node repository.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"context"
	"log/slog"
	"sync"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// nodeLister is the narrow read this adapter needs from storage. It is declared
// here rather than taking repository.NodeRepository so the adapter depends on
// one method, not the whole contract.
type nodeLister interface {
	List(ctx context.Context) ([]domain.ProviderNode, error)
}

// runtimeProviderIndex resolves a provider identifier against the embedded
// registry plus the currently stored custom nodes.
//
// The overlay is rebuilt lazily on each lookup rather than kept in a cache,
// because a stale cache here reintroduces exactly the problem this adapter
// exists to solve: a node an operator just created that the gateway does not yet
// recognise. Node rows are few (a handful per deployment) and the read is a
// single indexed query, so the simplicity is worth more than the cache.
//
// A rebuild failure is logged and the embedded registry is used alone: refusing
// every provider lookup because the node table is briefly unavailable would take
// the whole gateway down for a feature that is additive.
type runtimeProviderIndex struct {
	embedded *registry.Index
	nodes    nodeLister
	logger   *slog.Logger

	// mu serialises rebuilds so concurrent requests do not each build their own
	// copy of the same overlay under load.
	mu sync.Mutex
}

// newRuntimeProviderIndex binds the embedded registry to the node store.
func newRuntimeProviderIndex(embedded *registry.Index, nodes nodeLister, logger *slog.Logger) *runtimeProviderIndex {
	if logger == nil {
		logger = slog.Default()
	}
	return &runtimeProviderIndex{embedded: embedded, nodes: nodes, logger: logger}
}

// Provider resolves an id, alias, or node prefix to its entry.
func (r *runtimeProviderIndex) Provider(name string) (registry.Provider, bool) {
	return r.overlay().Provider(name)
}

// All returns every entry, embedded and custom.
func (r *runtimeProviderIndex) All() []registry.Provider { return r.overlay().All() }

// Categories returns the measured category set of the overlaid index.
func (r *runtimeProviderIndex) Categories() []string { return r.overlay().Categories() }

// overlay returns the index a lookup should read: the embedded registry with
// the currently stored custom nodes applied when that succeeds, and the
// embedded registry alone when it does not.
//
// A custom node's own prefix and id are resolved first inside the overlay: a
// node is the operator's explicit intent, and its prefix is refused at creation
// when it would collide with a registry identifier, so the order cannot shadow
// a built-in provider.
func (r *runtimeProviderIndex) overlay() *registry.Index {
	nodes, err := r.loadNodes(context.Background())
	if err != nil {
		r.logger.Warn("membaca provider node gagal; memakai registry saja", "error", err)
		return r.embedded
	}

	r.mu.Lock()
	overlaid, buildErr := r.embedded.WithCustom(nodes...)
	r.mu.Unlock()
	if buildErr != nil {
		// A stored node whose prefix collides (created before that rule, or by a
		// direct database write) must not make the whole registry unusable.
		r.logger.Error("membangun index dengan provider node gagal", "error", buildErr)
		return r.embedded
	}
	return overlaid
}

// loadNodes reads the stored nodes.
func (r *runtimeProviderIndex) loadNodes(ctx context.Context) ([]registry.CustomNode, error) {
	rows, err := r.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.CustomNode, 0, len(rows))
	for _, row := range rows {
		out = append(out, registry.CustomNode{
			ID:      row.ID(),
			Name:    row.Name(),
			Prefix:  row.Prefix(),
			APIType: row.APIType(),
			BaseURL: row.BaseURL(),
		})
	}
	return out, nil
}

// assertNodeLister keeps repository.NodeRepository satisfying the narrow port,
// so a change to either becomes a compile error here rather than at the call
// site.
var _ nodeLister = (repository.NodeRepository)(nil)
