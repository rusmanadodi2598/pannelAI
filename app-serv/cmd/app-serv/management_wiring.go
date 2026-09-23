// Command app-serv wires the P1 management and data-plane dependencies.
//
// @file      cmd/app-serv/management_wiring.go
// @for       Builds the P1 repositories, services, and handlers, and assembles
//
//	the router dependencies from them.
//
// @uses      internal/config, internal/dataplane, internal/domain,
//
//	internal/provider, internal/registry,
//	internal/repository/{postgres,redis}, internal/service,
//	internal/router, pgxpool, redis. The handlers themselves are built
//	in management_handlers.go.
//
// @reason    AGENTS.md §1.5 makes this file wiring only: it constructs the
//
//	dependency graph and nothing else, so no business rule hides in the
//	composition root. It is separate from main.go because the P1 surface
//	is a dozen services; one file would blow the §1.1 line budget and mix
//	the boot sequence with the graph.
//
//	Adapters that already exist in their own package are used as they
//	are (`dataplane.NewCatalogLookup`, `dataplane.NewMediaTransport`); only
//	what genuinely has no implementation is written here, which is why
//	this file is mostly construction rather than code.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// buildManagement assembles the P1 graph.
func buildManagement(
	cfg config.Config,
	pool *pgxpool.Pool,
	client redis.UniversalClient,
	index *registry.Index,
	connectors *provider.Connectors,
	keys repository.GatewayKeyRepository,
) (managementDeps, error) {
	// Settings, the egress policy, and the sealer are the process-wide
	// collaborators every builder below reads, so they are built together
	// (see foundation_wiring.go).
	settingsSvc, egress, sealer, err := buildFoundation(cfg, pool)
	if err != nil {
		return managementDeps{}, err
	}

	// Repositories.
	nodeRepo := postgres.NewNodeRepository(pool)
	endpointRepo := postgres.NewEndpointRepository(pool)
	catalogRepo := postgres.NewModelCatalogRepository(pool)
	comboRepo := postgres.NewComboRepository(pool)
	visionRepo := postgres.NewVisionAdapterRepository(pool)
	usageRepo := postgres.NewUsageRepository(pool)
	quotaRepo := postgres.NewQuotaRepository(pool)
	logRepo := postgres.NewLogRepository(pool)

	// A node's model list comes from the node's own upstream (draft 017 §4.2),
	// so the read is wired before the index that carries its answer: the index
	// attaches each node's list while it synthesizes the overlay.
	nodeModels := newNodeModelSource(
		newNodeTargetLookup(nodeRepo), connectors, egress.Guard,
		newNodeCredentialSource(endpointRepo, sealer), nodeModelCacheTTL,
	)

	// The runtime index overlays stored nodes on the embedded registry, so a node
	// created at runtime is resolvable as a provider id.
	runtimeIndex := newRuntimeProviderIndex(index, nodeRepo, nodeModels, slog.Default())

	// The catalog must exist before the combo and vision services, which resolve
	// their refs through it. It reads the runtime overlay like the rest of the
	// management graph: a model the operator registers under a custom node has
	// to reach the catalog, or the node's models are routable but invisible.
	catalogSvc, err := service.NewModelCatalogService(service.ModelCatalogServiceDeps{
		Index: runtimeIndex, Repo: catalogRepo, Combos: comboRepo,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: model catalog: %w", err)
	}

	// The probe adapter reaches the upstream through the plugin seam, so no part
	// of the graph special-cases a provider id. It reads the runtime overlay for
	// the same reason the data plane does: an endpoint under a custom node must
	// probe as that node. §7.4's stateless credential checks share it: they ask
	// the same question before a row exists (draft 017 §4.6).
	prober := newHTTPEndpointProber(runtimeIndex, connectors, egress.Guard)
	validationSvc, err := service.NewCredentialValidationService(prober)
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: credential validation: %w", err)
	}

	providerSvc, err := service.NewProviderService(service.ProviderServiceDeps{
		Index: runtimeIndex, Counts: endpointRepo, Source: nodeModels,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: providers: %w", err)
	}
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: providers: %w", err)
	}

	endpointSvc, err := service.NewEndpointService(service.EndpointServiceDeps{
		Store: endpointRepo, Index: runtimeIndex, Sealer: sealer, Prober: prober,
		Proxies: proxyPoolFinder{proxies: postgres.NewProxyRepository(pool)},
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: endpoints: %w", err)
	}

	nodeSvc, err := service.NewNodeService(service.NodeServiceDeps{
		Store: nodeRepo, Index: runtimeIndex, Counts: endpointRepo, Prober: prober,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: provider nodes: %w", err)
	}

	comboSvc, err := service.NewComboService(service.ComboServiceDeps{
		Repo: comboRepo, Catalog: catalogSvc, Rotation: redisrepo.NewComboRotationStore(client),
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: combos: %w", err)
	}

	// The capability predicate is the registry's ported vision table: §7.8 refuses
	// a model that cannot read images, and the registry carries no modality data,
	// so the answer comes from internal/registry/capability.go rather than from a
	// guess at this call site.
	visionSvc, err := service.NewVisionAdapterService(service.VisionAdapterServiceDeps{
		Repo: visionRepo, Catalog: catalogSvc, Capable: visionCapabilityCheck,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: vision adapter: %w", err)
	}

	// The augmenter is the engine's view of the adapter: the pipeline asks the
	// seam, and this is what answers it (SPEC-API-001 §7.8).
	augmenter, err := service.NewVisionAugmenter(service.VisionAugmenterDeps{
		Adapter:  visionSvc,
		Capable:  visionCapabilityCheck,
		Rotation: redisrepo.NewVisionRotationStore(client),
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: vision augmenter: %w", err)
	}

	// The usage, quota, and log services share repositories, so they are built
	// together (observability_wiring.go).
	obs, err := buildObservability(usageRepo, quotaRepo, endpointRepo, logRepo, settingsSvc, client)
	if err != nil {
		return managementDeps{}, err
	}

	flusher, retention, err := buildWorkers(client, quotaRepo, obs.Log)
	if err != nil {
		return managementDeps{}, err
	}

	// The §7.7 combo test probes through the engine, and the engine asks the combo
	// service for the round-robin order, so the probe is its own service — a
	// method on that one would make the cycle real.
	// §7.10 media providers and the §7.15 data plane, from the repositories the
	// management side writes through, so a value written by one path is readable
	// by the other.
	mediaPlane, err := buildMediaAndDataPlane(managementDataPlaneInputs{
		Config: cfg, Pool: pool, Redis: client, Index: runtimeIndex,
		Endpoints: endpointRepo, Counts: endpointRepo, Combos: comboRepo, ComboOrder: comboSvc,
		Catalog: catalogRepo, Keys: keys, Sealer: sealer, Connectors: connectors,
		Client: egress.Client, Settings: settingsSvc, Observability: obs, Vision: augmenter,
	})
	if err != nil {
		return managementDeps{}, err
	}
	mediaHandler, plane := mediaPlane.MediaHandler, mediaPlane.Plane

	comboTestSvc, err := service.NewComboTestService(comboSvc, plane.Engine)
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: combo test: %w", err)
	}

	tokenSaverSvc, err := service.NewTokenSaverService(service.TokenSaverServiceDeps{Settings: settingsSvc})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: token saver: %w", err)
	}

	// §7.11 proxy pools, over the process's shared egress guard.
	proxyHandler, err := buildProxies(cfg, pool, sealer, egress.Guard)
	if err != nil {
		return managementDeps{}, err
	}

	oauthHandler, refreshWorker, err := buildOAuth(cfg, runtimeIndex, endpointRepo, client, sealer, egress.Client)
	if err != nil {
		return managementDeps{}, err
	}
	return buildManagementHandlers(managementHandlerInputs{
		Provider: providerSvc, Validation: validationSvc, Endpoint: endpointSvc, Node: nodeSvc, Catalog: catalogSvc,
		Combo: comboSvc, ComboTest: comboTestSvc, Proxy: proxyHandler, Media: mediaHandler,
		Plane: plane, Vision: visionSvc, TokenSaver: tokenSaverSvc, Settings: settingsSvc,
		OAuth: oauthHandler, Observability: obs, Flusher: flusher, Retention: retention,
		RefreshWorker: refreshWorker,
	})
}
