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
//	internal/service, internal/tokensaver, net/http, redis.
//
// @reason    The data plane declares narrow ports and must not import a driver or a
//
//	service (AGENTS.md §1.5), so the composition root is where those ports
//	meet their implementations. No adapter type is declared here, because
//	none is needed: the gateway key repository answers the authentication
//	and key-use seams, the settings service answers the require-key seam,
//	and the usage and log services answer the accounting seams — each by a
//	method written for its own use, which is what makes the seams narrow
//	enough to satisfy without translation.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	redisrepo "github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository/redis"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/tokensaver"
)

// dataPlane is the assembled data plane: the chat, embeddings, and media
// services that share one engine, plus the engine itself — the §7.7 combo test
// probes through it, and rebuilding a second pipeline for that route would be a
// second pipeline to keep in step.
type dataPlane struct {
	Chat       *service.ChatService
	Embeddings *service.EmbeddingsService
	Media      *service.MediaCallService
	// SystemOne serves the decision models declaring `kind: systemone`, whose
	// payload is the provider's own vocabulary (SPEC-API-001 §7.15).
	SystemOne *service.SystemOneService
	Engine    *dataplane.Engine
	// Caller is the one media HTTP transport, shared so the embeddings and
	// media routes draw on the same connection pool (§1.7).
	Caller dataplane.MediaCaller
}

// dataPlaneInputs are the collaborators the data plane is built from. They are
// passed as one value rather than eight parameters so a caller cannot transpose
// two of them silently, which is the failure a long positional list invites.
type dataPlaneInputs struct {
	Config config.Config
	// Index is the runtime overlay, not the boot-frozen registry: a provider
	// node created after boot has to be routable by the next request.
	Index     dataplane.ProviderRegistry
	Endpoints repository.EndpointRepository
	Combos    repository.ComboRepository
	// ComboOrder is the §7.7 order seam, satisfied by the combo service: the
	// round-robin rule and its Redis state stay in one place, and the engine
	// asks for the order rather than reading the state.
	ComboOrder dataplane.ComboOrderer
	Catalog    repository.ModelCatalogRepository
	Keys       repository.GatewayKeyRepository
	Sealer     service.CredentialSealer
	Connectors *provider.Connectors
	// Client is the guarded upstream client from egress_wiring.go. Chat, media,
	// and embeddings all draw on it, so every upstream dial goes through the one
	// guard and the one connection pool (OWASP A01, AGENTS.md §1.7).
	Client   *http.Client
	Redis    redis.UniversalClient
	Settings *service.SettingsService
	Usage    *service.UsageService
	// Logs writes the request-log half of the data plane's accounting pair. It
	// is passed in rather than built here because the management side reads the
	// same rows through the same service.
	Logs service.RequestLogRecorder
	// Vision is the §7.8 seam the engine consults for image-bearing requests.
	Vision dataplane.VisionAugmenter
	// MediaOverrides is the §7.10 seam the media routes read a stored base
	// URL through. It is passed in rather than built here because the same
	// service answers the management routes.
	MediaOverrides service.MediaOverrideReader
	// MediaIndex is the same runtime overlay, typed for the media service's
	// own index port.
	MediaIndex service.ProviderIndex
	// Quotas is the §7.12 budget gate the selector consults before picking an
	// endpoint, and the counter the accounting sites advance. It is passed in
	// because the management side reads the same rows through the same service.
	Quotas *service.QuotaService
	// ActiveRequests marks one provider as in flight while its call runs, which
	// is what the §7.12 live stream draws. It is passed in because the live
	// service reads the same store, so one tracker instance serves the stream and
	// every plane that writes to it.
	ActiveRequests dataplane.ActiveRequests
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
		// The credential rotation policy (§7.14) resolved per provider by the
		// settings service: fill-first or round-robin with the provider's own
		// sticky override over the global default. Rotation is an optimisation,
		// so a policy read that fails degrades to priority order in the
		// selector rather than failing the request.
		Strategies: in.Settings,
		// §7.12: an endpoint whose stored budget cap is spent is skipped, which
		// is the enforcement half of the quota surface. The quota service
		// satisfies the seam directly, so no adapter is written for it.
		Gate: in.Quotas,
		// A credential-free provider with no stored endpoint answers on a
		// synthesized one, so a free lane is usable the moment its provider is
		// listed (draft 029 F8; the reference injects the same connection).
		// The index is the embedded registry overlaid with stored custom nodes,
		// so a node the operator created is answerable by the same rule.
		Registry: in.Index,
	})
	if err != nil {
		return dataPlane{}, err
	}

	transport, err := dataplane.NewTransport(dataplane.TransportDeps{Connectors: in.Connectors, Client: in.Client})
	if err != nil {
		return dataPlane{}, err
	}

	saver, err := tokensaver.NewApplierWithClient(in.Settings, in.Client, dataplane.TokenSaverTranslator{})
	if err != nil {
		return dataPlane{}, err
	}

	engine, err := dataplane.NewEngine(dataplane.EngineDeps{
		Resolver: resolver, Selector: selector, Transport: transport, Vision: in.Vision,
		TokenSaver: saver,
		// The chat plane opens one marker per relay leg, so a provider the engine
		// is calling right now is a node the live drawing lights (SPEC-UI-001
		// §6.5). The same tracker instance is handed to the media and embeddings
		// services below, so all three planes write to one set.
		ActiveRequests: in.ActiveRequests,
		// The combo service owns the round-robin rule and the counter it
		// advances, so the engine asks it for the order instead of reading the
		// rotation state itself. It satisfies the seam directly.
		ComboOrder: in.ComboOrder,
	})
	if err != nil {
		return dataPlane{}, err
	}

	// One counter instance for every accounting site, so the chat, media, and
	// embeddings planes advance the same Redis keys and cannot disagree about
	// which windows a call bills against (§7.12, register G22). It is built here
	// because this is the only function that owns both the Redis client and the
	// data plane's services.
	quotas := service.NewQuotaCounter(redisrepo.NewQuotaCounterStore(in.Redis), slog.Default())

	// The settings and usage services satisfy their seams directly, so no
	// adapter is written for either.
	chat, err := service.NewChatService(service.ChatServiceDeps{
		Engine:   engine,
		Keys:     in.Keys,
		Settings: in.Settings,
		Usage:    in.Usage,
		// The log service answers the §7.13 seam directly, so one chat call
		// leaves the same accounting pair a media call does (register G18).
		Logs: in.Logs,
		// The quota counters advance by the tokens the upstream billed, so the
		// window the panel reads is what this gateway actually served (§7.12).
		Quotas: quotas,
		// The key repository answers the use-counter seam too: every call the
		// §4 rule admits advances that key's request_count (SPEC-API-001 §7.3).
		KeyUse: in.Keys,
		// The request id comes from the router's context, so a usage row and the
		// log line for one request share an identifier (SPEC-API-001 §4). It is a
		// function value so the service layer never imports the HTTP layer.
		RequestID: router.RequestIDFrom,
	})
	if err != nil {
		return dataPlane{}, err
	}

	// The media and embeddings planes are built beside the engine they route
	// through (see dataplane_media_wiring.go), because they share its resolver,
	// selector, and quota counter.
	embeddings, media, systemone, caller, err := buildMediaPlanes(in, engine, quotas)
	if err != nil {
		return dataPlane{}, err
	}
	return dataPlane{
		Chat: chat, Embeddings: embeddings, Media: media, SystemOne: systemone,
		Engine: engine, Caller: caller,
	}, nil
}
