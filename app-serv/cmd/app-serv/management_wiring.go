// Command app-serv wires the P1 management and data-plane dependencies.
//
// @file      cmd/app-serv/management_wiring.go
// @for       Builds the P1 repositories, services, and handlers, and assembles
//
//	the router dependencies from them.
//
// @uses      internal/config, internal/dataplane, internal/domain,
//
//	internal/handler, internal/provider, internal/registry,
//	internal/repository/{postgres,redis}, internal/service,
//	internal/router, pgxpool, redis.
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
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// managementDeps is the graph the router consumes.
type managementDeps struct {
	Provider      *handler.ProviderHandler
	Endpoint      *handler.EndpointHandler
	EndpointKey   *handler.EndpointKeyHandler
	EndpointBulk  *handler.EndpointBulkHandler
	Node          *handler.ProviderNodeHandler
	Model         *handler.ModelHandler
	Combo         *handler.ComboHandler
	VisionAdapter *handler.VisionAdapterHandler
	Usage         *handler.UsageHandler
	Quota         *handler.QuotaHandler
	Log           *handler.LogHandler
	Settings      *handler.SettingsHandler
	Chat          *handler.ChatHandler
	Embeddings    *handler.EmbeddingsHandler

	// QuotaFlusher is returned so the caller can run it after the server is
	// listening, rather than leaving a goroutine nothing supervises.
	QuotaFlusher *service.QuotaFlusher
}

// buildManagement assembles the P1 graph.
func buildManagement(
	cfg config.Config,
	pool *pgxpool.Pool,
	client redis.UniversalClient,
	index *registry.Index,
	connectors *provider.Connectors,
	keys repository.GatewayKeyRepository,
) (managementDeps, error) {
	// The sealer is created once and shared: sealing and opening must agree on
	// the key, so a second instance would be a second source of truth for it.
	sealer, err := domain.NewSealer([]byte(cfg.EncryptionKey))
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: %w", err)
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
	settingsRepo := postgres.NewSettingsRepository(pool)

	// The runtime index overlays stored custom nodes on the embedded registry,
	// so a node created at runtime is resolvable as a provider id.
	runtimeIndex := newRuntimeProviderIndex(index, nodeRepo, slog.Default())

	// Settings first: the log service reads the capture policy from it.
	settingsSvc, err := service.NewSettingsService(service.SettingsServiceDeps{Repo: settingsRepo})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: settings: %w", err)
	}

	// The catalog must exist before the combo and vision services, which resolve
	// their refs through it.
	catalogSvc, err := service.NewModelCatalogService(service.ModelCatalogServiceDeps{
		Index: index, Repo: catalogRepo, Combos: comboRepo,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: model catalog: %w", err)
	}

	// The probe adapter reaches the upstream through the plugin seam, so no part
	// of the graph special-cases a provider id. It reads the runtime overlay for
	// the same reason the data plane does: an endpoint under a custom node must
	// probe as that node.
	prober := newHTTPEndpointProber(runtimeIndex, connectors)

	providerSvc, err := service.NewProviderService(service.ProviderServiceDeps{
		Index: runtimeIndex, Counts: endpointRepo,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: providers: %w", err)
	}

	endpointSvc, err := service.NewEndpointService(service.EndpointServiceDeps{
		Store: endpointRepo, Index: runtimeIndex, Sealer: sealer, Prober: prober,
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

	// The capability predicate is left to the service's safe default: the
	// registry does not yet carry input modalities, so a predicate written here
	// would be a guess the API then acts on. This is the wiring point where the
	// real one belongs once the registry carries the data.
	visionSvc, err := service.NewVisionAdapterService(service.VisionAdapterServiceDeps{
		Repo: visionRepo, Catalog: catalogSvc,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: vision adapter: %w", err)
	}

	usageSvc, err := service.NewUsageService(service.UsageServiceDeps{
		Usage: usageRepo, Logs: logRepo, Settings: settingsSvc,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: usage: %w", err)
	}

	quotaSvc, err := service.NewQuotaService(service.QuotaServiceDeps{Quotas: quotaRepo, Usage: usageRepo})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: quotas: %w", err)
	}

	logSvc, err := service.NewLogService(service.LogServiceDeps{
		Logs: logRepo, Settings: settingsSvc, Console: redisrepo.NewConsoleBuffer(client),
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: logs: %w", err)
	}

	flusher, err := service.NewQuotaFlusher(
		redisrepo.NewQuotaCounterStore(client), quotaRepo, service.DefaultQuotaFlushPolicy(), slog.Default(),
	)
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: quota flusher: %w", err)
	}

	// The data plane is assembled from the same repositories the management side
	// writes through, so a value written by one path is readable by the other.
	// buildDataPlane owns that construction; this function only feeds it.
	//
	// The index handed over is the runtime overlay, not the embedded registry:
	// the catalog route accepts a custom node as a provider_id, so the router
	// must resolve the same ids the route does.
	plane, err := buildDataPlane(dataPlaneInputs{
		Config: cfg, Index: runtimeIndex, Endpoints: endpointRepo, Combos: comboRepo,
		Catalog: catalogRepo, Keys: keys, Sealer: sealer, Connectors: connectors,
		Redis: client, Settings: settingsSvc, Usage: usageSvc,
	})
	if err != nil {
		return managementDeps{}, fmt.Errorf("management wiring: data plane: %w", err)
	}

	return managementDeps{
		Provider:      handler.NewProviderHandler(providerSvc),
		Endpoint:      handler.NewEndpointHandler(endpointSvc),
		EndpointKey:   handler.NewEndpointKeyHandler(endpointSvc),
		EndpointBulk:  handler.NewEndpointBulkHandler(endpointSvc),
		Node:          handler.NewProviderNodeHandler(nodeSvc),
		Model:         handler.NewModelHandler(catalogSvc),
		Combo:         handler.NewComboHandler(comboSvc),
		VisionAdapter: handler.NewVisionAdapterHandler(visionSvc),
		Usage:         handler.NewUsageHandler(usageSvc),
		Quota:         handler.NewQuotaHandler(quotaSvc),
		Log:           handler.NewLogHandler(logSvc),
		Settings:      handler.NewSettingsHandler(settingsSvc),
		Chat:          handler.NewChatHandler(plane.Chat),
		Embeddings:    handler.NewEmbeddingsHandler(plane.Embeddings, plane.Chat),
		QuotaFlusher:  flusher,
	}, nil
}

// runQuotaFlusher runs the flush worker until the context is cancelled.
//
// The goroutine recovers from a panic and has an explicit termination condition
// (AGENTS.md §1.6): it returns when ctx is cancelled, so shutdown leaves nothing
// running.
func runQuotaFlusher(ctx context.Context, flusher *service.QuotaFlusher) {
	if flusher == nil {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic saat menjalankan quota flusher", "panic", recovered)
			}
		}()
		flusher.Run(ctx)
	}()
}
