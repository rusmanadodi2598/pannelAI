// Command app-serv wires the P1 data-plane dependencies.
//
// @file      cmd/app-serv/dataplane_wiring.go
// @for       Builds the data plane's resolver, selector, transport, engine, and
//
//	the chat and embeddings services on top of them.
//
// @uses      internal/dataplane, internal/provider, internal/registry,
//
//	internal/repository, internal/repository/redis, internal/router,
//	internal/service, redis.
//
// @reason    The data plane declares narrow ports and must not import a driver or a
//
//	service (AGENTS.md §1.5), so the composition root is where those ports
//	meet their implementations. No adapter type is declared here, because
//	none is needed: the gateway key repository already answers the
//	authentication seam, the settings service already answers the
//	require-key seam, and the usage service already answers the recorder
//	seam — each by a method written for its own use, which is what makes
//	the seams narrow enough to satisfy without translation.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// dataPlane is the assembled data plane: the chat and embeddings services that
// share one engine.
type dataPlane struct {
	Chat       *service.ChatService
	Embeddings *service.EmbeddingsService
}

// dataPlaneInputs are the collaborators the data plane is built from. They are
// passed as one value rather than eight parameters so a caller cannot transpose
// two of them silently, which is the failure a long positional list invites.
type dataPlaneInputs struct {
	Config     config.Config
	Index      *registry.Index
	Endpoints  repository.EndpointRepository
	Combos     repository.ComboRepository
	Catalog    repository.ModelCatalogRepository
	Keys       repository.GatewayKeyRepository
	Sealer     service.CredentialSealer
	Connectors *provider.Connectors
	Redis      redis.UniversalClient
	Settings   *service.SettingsService
	Usage      *service.UsageService
}

// buildDataPlane assembles the resolver, selector, transport, and engine, then the
// services that use them.
//
// The order is not arbitrary: the resolver needs the catalog lookup, the selector
// needs the endpoints and the credential opener, and the engine needs all three.
// Building them together keeps that dependency visible instead of scattered
// across constructors that each reach for a global.
func buildDataPlane(in dataPlaneInputs) (dataPlane, error) {
	lookup, err := dataplane.NewCatalogLookup(in.Combos, in.Catalog)
	if err != nil {
		return dataPlane{}, err
	}
	resolver, err := dataplane.NewResolver(in.Index, lookup)
	if err != nil {
		return dataPlane{}, err
	}

	selector, err := dataplane.NewSelector(dataplane.SelectorDeps{
		Endpoints: in.Endpoints,
		// The opener is the same sealer the management side writes with: sealing
		// and opening must share one implementation, or a value written by one
		// path becomes unreadable by the other.
		Opener: in.Sealer,
		// The cursor lives in Redis so the rotation position is shared by every
		// replica; a per-process cursor would make distribution depend on which
		// instance answered.
		Cursor: redisrepo.NewCursorStore(in.Redis),
		// The global default. A combo with its own limit overrides it in the
		// strategy layer, not here.
		StickyLimit: in.Config.DataPlaneStickyLimit,
	})
	if err != nil {
		return dataPlane{}, err
	}

	transport, err := dataplane.NewTransport(dataplane.TransportDeps{Connectors: in.Connectors})
	if err != nil {
		return dataPlane{}, err
	}

	engine, err := dataplane.NewEngine(dataplane.EngineDeps{
		Resolver: resolver, Selector: selector, Transport: transport,
	})
	if err != nil {
		return dataPlane{}, err
	}

	// The settings and usage services satisfy their seams directly, so no
	// adapter is written for either.
	chat, err := service.NewChatService(service.ChatServiceDeps{
		Engine:   engine,
		Keys:     in.Keys,
		Settings: in.Settings,
		Usage:    in.Usage,
		// The request id comes from the router's context, so a usage row and the
		// log line for one request share an identifier (SPEC-API-001 §4). It is a
		// function value so the service layer never imports the HTTP layer.
		RequestID: router.RequestIDFrom,
	})
	if err != nil {
		return dataPlane{}, err
	}

	embeddings, err := service.NewEmbeddingsService(service.EmbeddingsServiceDeps{
		Engine: engine,
		// The package's own HTTP implementation, which already carries the §1.7
		// pool limits and the §1.6 deadlines.
		Caller: dataplane.NewMediaTransport(nil),
	})
	if err != nil {
		return dataPlane{}, err
	}
	return dataPlane{Chat: chat, Embeddings: embeddings}, nil
}
