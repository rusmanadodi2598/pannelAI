// Command app-serv wires the P1 data-plane dependencies.
//
// @file      cmd/app-serv/dataplane_inputs.go
// @for       The collaborators the data plane is built from, as one value.
//
// @uses      internal/config, internal/dataplane, internal/provider,
//
//	internal/repository, internal/service, net/http, redis.
//
// @reason    A long positional parameter list is where two same-typed arguments
//
//	get transposed silently, so the collaborators travel as one named
//	struct. It lives in its own file because the struct is a declaration
//	rather than wiring, and because dataplane_wiring.go stays inside the
//	AGENTS.md §1.1 line budget with it here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package main

import (
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

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
	Client *http.Client
	// Routes plans the proxy pool's attempts per destination (PORT 008). The
	// composition root builds it beside the proxy handler so the plan draws on
	// the same rows the operator edits; a nil value keeps the shared client's
	// own routing, which is what the tests and any pre-pool wiring rely on.
	Routes   dataplane.ProxyRoutePlanner
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
