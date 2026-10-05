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
// @stability stable
// @since     2026-09-17
package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// nodeLister is the narrow read this adapter needs from storage. It is declared
// here rather than taking repository.NodeRepository so the adapter depends on
// one method, not the whole contract.
type nodeLister interface {
	List(ctx context.Context) ([]domain.ProviderNode, error)
}

const (
	// overlayCacheTTL bounds how long a built overlay is believed before it is
	// re-read, and how long a failed rebuild is retried after.
	overlayCacheTTL = 30 * time.Second
	// overlayRebuildTimeout bounds one node read plus registry rebuild.
	overlayRebuildTimeout = 5 * time.Second
)

// runtimeProviderIndex resolves a provider identifier against the embedded
// registry plus the currently stored custom nodes. The overlay is cached for
// overlayCacheTTL and rebuilt lazily: rebuilding per lookup would cost one node
// query plus a registry rebuild on every request. InvalidateNodeOverlay runs on
// every node write, so a created or deleted node is visible on the next lookup;
// the TTL only backstops a write made outside this process, a direct database
// change. A failed rebuild keeps serving the last good overlay rather than the
// embedded registry alone, so a node-table outage cannot unlist every provider.
type runtimeProviderIndex struct {
	embedded *registry.Index
	nodes    nodeLister
	models   service.NodeModelSource
	logger   *slog.Logger

	// mu serialises rebuilds so concurrent requests do not each build their own
	// copy of the same overlay under load.
	mu sync.Mutex

	// clock separates "how long a built overlay is believed" from wall time, so
	// a test states the window instead of sleeping through it.
	clock func() time.Time

	cached   *registry.Index
	cachedAt time.Time
}

// newRuntimeProviderIndex binds the embedded registry to the node store.
//
// models may be nil, in which case a node is synthesized with the models it
// declares and nothing is dialed: a deployment that wires no source still
// resolves every node, it just cannot read one's upstream list.
func newRuntimeProviderIndex(embedded *registry.Index, nodes nodeLister, models service.NodeModelSource, logger *slog.Logger) *runtimeProviderIndex {
	if logger == nil {
		logger = slog.Default()
	}
	return &runtimeProviderIndex{embedded: embedded, nodes: nodes, models: models, logger: logger, clock: time.Now}
}

// Provider resolves an id, alias, or node prefix to its entry.
func (r *runtimeProviderIndex) Provider(name string) (registry.Provider, bool) {
	return r.overlay().Provider(name)
}

// Model resolves a declared model inside an entry, custom nodes included.
func (r *runtimeProviderIndex) Model(providerName, modelID string) (registry.Model, bool) {
	return r.overlay().Model(providerName, modelID)
}

// All returns every entry, embedded and custom.
func (r *runtimeProviderIndex) All() []registry.Provider { return r.overlay().All() }

// Categories returns the measured category set of the overlaid index.
func (r *runtimeProviderIndex) Categories() []string { return r.overlay().Categories() }

// overlay returns the index a lookup should read: the cached overlay while it
// is fresh, a rebuild when it is not, and the last good overlay (or the embedded
// registry, on the very first build) when the rebuild fails.
//
// A custom node's own prefix and id are resolved first inside the overlay: a
// node is the operator's explicit intent, and its prefix is refused at creation
// when it would collide with a registry identifier, so the order cannot shadow
// a built-in provider.
func (r *runtimeProviderIndex) overlay() *registry.Index {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cached != nil && r.clock().Sub(r.cachedAt) < overlayCacheTTL {
		return r.cached
	}

	ctx, cancel := context.WithTimeout(context.Background(), overlayRebuildTimeout)
	defer cancel()
	nodes, err := r.loadNodes(ctx)
	if err != nil {
		r.logger.Warn("membaca provider node gagal; memakai overlay terakhir", "error", err)
		return r.staleAfterFailure()
	}

	overlaid, buildErr := r.embedded.WithCustom(nodes...)
	if buildErr != nil {
		// A stored node whose prefix collides (created before that rule, or by a
		// direct database write) must not make the whole registry unusable.
		r.logger.Error("membangun index dengan provider node gagal", "error", buildErr)
		return r.staleAfterFailure()
	}
	r.cached, r.cachedAt = overlaid, r.clock()
	return r.cached
}

// staleAfterFailure keeps serving the overlay a lookup can still resolve against
// when a rebuild fails: the last good overlay if one was built, the embedded
// registry alone on the very first read. It also stamps the retry clock so a
// sustained outage backs off for a full window instead of re-querying per request.
func (r *runtimeProviderIndex) staleAfterFailure() *registry.Index {
	if r.cached == nil {
		r.cached = r.embedded
	}
	r.cachedAt = r.clock()
	return r.cached
}

// InvalidateNodeOverlay drops the cached overlay so the next lookup re-reads the
// node store. The service calls it after every node write, so a created or deleted
// node is visible without waiting for the TTL window to close.
func (r *runtimeProviderIndex) InvalidateNodeOverlay() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cached = nil
}

// loadNodes reads the stored nodes, each carrying its own model list.
//
// The mapping goes through service.NodeCustomNode, the one place that knows how
// a stored node becomes a registry node. The model list is injected here rather
// than resolved by each consumer because the detail route, the catalog and the
// data plane all read Provider.Models from the index. A source that fails leaves
// the node its declared list: an upstream that is down must not make the node
// unresolvable.
func (r *runtimeProviderIndex) loadNodes(ctx context.Context) ([]registry.CustomNode, error) {
	rows, err := r.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.CustomNode, 0, len(rows))
	for _, row := range rows {
		node := service.NodeCustomNode(row)
		if r.models != nil {
			if list, listErr := r.models.ListNodeModels(ctx, row.ID()); listErr == nil {
				node.Models = list.Models
			}
		}
		out = append(out, node)
	}
	return out, nil
}

// assertNodeLister keeps repository.NodeRepository satisfying the narrow port,
// so a change to either becomes a compile error here rather than at the call
// site.
var _ nodeLister = (repository.NodeRepository)(nil)
