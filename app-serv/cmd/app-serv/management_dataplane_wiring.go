// Command app-serv builds the media and data-plane halves of the graph.
//
// @file      cmd/app-serv/management_dataplane_wiring.go
// @for       The §7.10 media provider services and the §7.15 data plane, built
//
//	from the repositories the management side writes through.
//
// @uses      internal/domain, internal/registry, internal/repository/postgres,
//
//	internal/service, pgxpool, redis.
//
// @reason    buildManagement was at the AGENTS.md §1.1 warning threshold, and this
//
//	block is the part that grows with every data-plane route. It is a
//	distinct job — it builds the plane that serves client traffic, not the
//	management graph — so it is separate rather than trimmed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"fmt"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/postgres"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// mediaAndDataPlane is what the §7.10/§7.15 build produces.
type mediaAndDataPlane struct {
	MediaService *service.MediaProviderService
	MediaHandler *handler.MediaProviderHandler
	Plane        dataPlane
}

// buildMediaAndDataPlane builds the media provider services and the data plane.
func buildMediaAndDataPlane(in managementDataPlaneInputs) (mediaAndDataPlane, error) {
	// §7.10 media providers, resolved before the data plane because the media and
	// embeddings routes read the same overrides through this service.
	mediaSvc, mediaHandler, err := buildMediaProviders(in.Pool, in.Index, in.Counts)
	if err != nil {
		return mediaAndDataPlane{}, err
	}

	plane, err := buildDataPlane(dataPlaneInputs{
		Config: in.Config, Index: in.Index, Endpoints: in.Endpoints, Combos: in.Combos,
		ComboOrder: in.ComboOrder, Catalog: in.Catalog, Keys: in.Keys, Sealer: in.Sealer,
		Connectors: in.Connectors, Client: in.Client, Routes: in.Routes, Redis: in.Redis,
		Settings: in.Settings,
		Usage:    in.Observability.Usage, Vision: in.Vision,
		ActiveRequests: in.Observability.Active, Logs: in.Observability.Log,
		Quotas: in.Observability.Quota, MediaOverrides: mediaSvc, MediaIndex: in.Index,
	})
	if err != nil {
		return mediaAndDataPlane{}, fmt.Errorf("management wiring: data plane: %w", err)
	}
	return mediaAndDataPlane{MediaService: mediaSvc, MediaHandler: mediaHandler, Plane: plane}, nil
}

// managementDataPlaneInputs is every collaborator the media and data-plane build
// needs. It is a struct rather than a parameter list because the set is a dozen
// wide and a positional call would be a row of same-typed arguments.
type managementDataPlaneInputs struct {
	Config     config.Config
	Pool       *pgxpool.Pool
	Redis      redis.UniversalClient
	Index      *runtimeProviderIndex
	Endpoints  repository.EndpointRepository
	Counts     service.EndpointCounterByProvider
	Combos     repository.ComboRepository
	ComboOrder dataplane.ComboOrderer
	Catalog    repository.ModelCatalogRepository
	Keys       repository.GatewayKeyRepository
	Sealer     service.CredentialSealer
	Connectors *provider.Connectors
	Client     *http.Client
	// Routes is the pool-driven proxy planner (PORT 008), built beside the
	// proxy handler so one pool serves both the management routes and the
	// data plane's walk.
	Routes        dataplane.ProxyRoutePlanner
	Settings      *service.SettingsService
	Observability observability
	Vision        *service.VisionAugmenter
}

var (
	_ = domain.NewValidationError
	_ = fmt.Errorf
	_ = postgres.NewComboRepository
	_ = redisrepo.NewComboRotationStore
)
