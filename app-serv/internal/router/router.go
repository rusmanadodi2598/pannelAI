// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router.go
// @for       Route registration for the v1 management and data planes.
// @uses      internal/handler, internal/schema, net/http, encoding/json.
// @reason    SPEC-API-001 §4 pins the /api/v1 prefix for every route and §9
//
//	fixes the layer flow ending at the router; registering each route
//	with its method makes the surface auditable at a glance and makes
//	a forgotten verb a startup-visible mistake, not a 405 at runtime.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-16
package router

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// APIVersion is the single version prefix for all routes (SPEC-API-001 §11.1).
const APIVersion = "/api/v1"

// Mux is the HTTP serve mux for app-serv.
type Mux struct {
	http.Handler
	routes []string
}

// Deps holds the handlers the router wires up.
//
// Every handler is a pointer so an unset one is detectable, and New refuses to
// register a route whose handler is missing: a nil handler would compile and
// then panic on the first request, which is the failure mode the composition
// root cannot see at boot.
type Deps struct {
	System           *handler.SystemHandler
	Auth             *handler.AuthHandler
	GatewayKey       *handler.GatewayKeyHandler
	Provider         *handler.ProviderHandler
	Endpoint         *handler.EndpointHandler
	EndpointKey      *handler.EndpointKeyHandler
	EndpointBulk     *handler.EndpointBulkHandler
	OAuth            *handler.OAuthHandler
	ProviderNode     *handler.ProviderNodeHandler
	ProviderValidate *handler.ProviderValidateHandler
	Model            *handler.ModelHandler
	Combo            *handler.ComboHandler
	ComboTest        *handler.ComboTestHandler
	Proxy            *handler.ProxyHandler
	MediaProvider    *handler.MediaProviderHandler
	Media            *handler.MediaHandler
	VisionAdapter    *handler.VisionAdapterHandler
	TokenSaver       *handler.TokenSaverHandler
	Usage            *handler.UsageHandler
	UsageLive        *handler.UsageLiveHandler
	Quota            *handler.QuotaHandler
	Log              *handler.LogHandler
	Settings         *handler.SettingsHandler
	Skills           *handler.SkillsHandler
	OpenAPI          *handler.OpenAPIHandler
	Changelog        *handler.ChangelogHandler
	Chat             *handler.ChatHandler
	Embeddings       *handler.EmbeddingsHandler
	TokenCount       *handler.TokenCountHandler
	RateLimiter      repository.RateLimiter
	RateLimitPerMin  int
}

// New registers every route and returns the assembled mux. Patterns carry the
// method (Go 1.22 ServeMux syntax) so the mux rejects a wrong verb before any
// handler sees it.
func New(deps Deps) *Mux {
	mux := &routeRecorder{ServeMux: http.NewServeMux()}

	// §7.1 System (public).
	mux.HandleFunc("GET "+APIVersion+"/health", deps.System.Health)
	mux.HandleFunc("GET "+APIVersion+"/version", deps.System.Version)

	// §7.2 Auth: status and login are public; mutations require a session.
	if deps.Auth != nil {
		mux.HandleFunc("POST "+APIVersion+"/auth/login", deps.Auth.Login)
		mux.HandleFunc("GET "+APIVersion+"/auth/status", deps.Auth.Status)
		mux.Handle("POST "+APIVersion+"/auth/logout", deps.Auth.RequireSession(http.HandlerFunc(deps.Auth.Logout)))
		mux.Handle("POST "+APIVersion+"/auth/change-password", deps.Auth.RequireSession(http.HandlerFunc(deps.Auth.ChangePassword)))
	}

	// §7.3 Gateway keys are management routes and therefore session-gated.
	gateway := func(next http.Handler) http.Handler {
		if deps.Auth == nil {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				schema.WriteError(w, domain.NewInternalError("authentication is unavailable"))
			})
		}
		return deps.Auth.RequireSession(next)
	}
	mux.Handle("GET "+APIVersion+"/gateway-keys", gateway(http.HandlerFunc(deps.GatewayKey.List)))
	mux.Handle("POST "+APIVersion+"/gateway-keys", gateway(http.HandlerFunc(deps.GatewayKey.Create)))
	mux.Handle("GET "+APIVersion+"/gateway-keys/{id}", gateway(http.HandlerFunc(deps.GatewayKey.Get)))
	mux.Handle("PATCH "+APIVersion+"/gateway-keys/{id}", gateway(http.HandlerFunc(deps.GatewayKey.Update)))
	mux.Handle("DELETE "+APIVersion+"/gateway-keys/{id}", gateway(http.HandlerFunc(deps.GatewayKey.Delete)))

	// §7.4 Providers and custom provider nodes (registry-backed, session-gated).
	mux.Handle("GET "+APIVersion+"/providers", gateway(http.HandlerFunc(deps.Provider.List)))
	mux.Handle("GET "+APIVersion+"/providers/{provider_id}", gateway(http.HandlerFunc(deps.Provider.Get)))
	mux.Handle("GET "+APIVersion+"/providers/{provider_id}/models", gateway(http.HandlerFunc(deps.Provider.Models)))
	mux.Handle("POST "+APIVersion+"/providers/{provider_id}/oauth/bulk", gateway(http.HandlerFunc(deps.Endpoint.ImportOAuth)))

	// §7.4 OAuth. Three of the four routes are management routes the panel
	// calls, so they share the session guard. The callback is deliberately
	// public: the provider redirects a browser to it, and a browser cannot
	// present the dashboard session cookie for that redirect, so gating it
	// would make every authorization fail at the last step. Its replay guard
	// is the single-use state (§4), not a session.
	if deps.OAuth != nil {
		mux.Handle("POST "+APIVersion+"/providers/{provider_id}/oauth/start", gateway(http.HandlerFunc(deps.OAuth.Start)))
		mux.HandleFunc("GET "+APIVersion+"/providers/{provider_id}/oauth/callback", deps.OAuth.Callback)
		mux.Handle("GET "+APIVersion+"/providers/{provider_id}/oauth/status", gateway(http.HandlerFunc(deps.OAuth.Status)))
		mux.Handle("POST "+APIVersion+"/providers/{provider_id}/oauth/refresh", gateway(http.HandlerFunc(deps.OAuth.Refresh)))
	}
	// §7.4's node and credential-check routes are registered together in
	// router_provider_nodes.go, because that section now carries seven routes
	// and registering them here would push this file past the §1.1 budget.
	registerProviderNodeRoutes(mux, deps, gateway)

	// §7.5 Upstream endpoints and their keys, including the batch onboarding
	// routes. All-or-nothing batch semantics live in the service (§8.1), so the
	// router only has to reach them.
	mux.Handle("GET "+APIVersion+"/endpoints", gateway(http.HandlerFunc(deps.Endpoint.List)))
	mux.Handle("POST "+APIVersion+"/endpoints", gateway(http.HandlerFunc(deps.Endpoint.Create)))
	mux.Handle("POST "+APIVersion+"/endpoints/bulk", gateway(http.HandlerFunc(deps.EndpointBulk.BulkCreate)))
	mux.Handle("GET "+APIVersion+"/endpoints/{id}", gateway(http.HandlerFunc(deps.Endpoint.Get)))
	mux.Handle("PATCH "+APIVersion+"/endpoints/{id}", gateway(http.HandlerFunc(deps.Endpoint.Update)))
	mux.Handle("DELETE "+APIVersion+"/endpoints/{id}", gateway(http.HandlerFunc(deps.Endpoint.Delete)))
	mux.Handle("POST "+APIVersion+"/endpoints/{id}/test", gateway(http.HandlerFunc(deps.Endpoint.Test)))
	mux.Handle("GET "+APIVersion+"/endpoints/{id}/keys", gateway(http.HandlerFunc(deps.EndpointKey.ListKeys)))
	mux.Handle("POST "+APIVersion+"/endpoints/{id}/keys", gateway(http.HandlerFunc(deps.EndpointKey.AddKey)))
	mux.Handle("POST "+APIVersion+"/endpoints/{id}/keys/bulk", gateway(http.HandlerFunc(deps.EndpointBulk.AddKeysBulk)))
	mux.Handle("PATCH "+APIVersion+"/endpoints/{id}/keys/{key_id}", gateway(http.HandlerFunc(deps.EndpointKey.UpdateKey)))
	mux.Handle("DELETE "+APIVersion+"/endpoints/{id}/keys/{key_id}", gateway(http.HandlerFunc(deps.EndpointKey.DeleteKey)))

	// §7.6–§7.8 Models, combos, and the vision adapter are management routes, so
	// they share the same session guard the gateway keys use.
	mux.Handle("GET "+APIVersion+"/models/catalog", gateway(http.HandlerFunc(deps.Model.Catalog)))
	mux.Handle("GET "+APIVersion+"/models/custom", gateway(http.HandlerFunc(deps.Model.CustomList)))
	mux.Handle("POST "+APIVersion+"/models/custom", gateway(http.HandlerFunc(deps.Model.CustomCreate)))
	mux.Handle("DELETE "+APIVersion+"/models/custom/{id}", gateway(http.HandlerFunc(deps.Model.CustomDelete)))
	mux.Handle("GET "+APIVersion+"/models/aliases", gateway(http.HandlerFunc(deps.Model.AliasesGet)))
	mux.Handle("PUT "+APIVersion+"/models/aliases", gateway(http.HandlerFunc(deps.Model.AliasesPut)))
	mux.Handle("GET "+APIVersion+"/models/disabled", gateway(http.HandlerFunc(deps.Model.DisabledGet)))
	mux.Handle("PUT "+APIVersion+"/models/disabled", gateway(http.HandlerFunc(deps.Model.DisabledPut)))
	mux.Handle("GET "+APIVersion+"/combos", gateway(http.HandlerFunc(deps.Combo.List)))
	mux.Handle("POST "+APIVersion+"/combos", gateway(http.HandlerFunc(deps.Combo.Create)))
	mux.Handle("GET "+APIVersion+"/combos/{id}", gateway(http.HandlerFunc(deps.Combo.Get)))
	mux.Handle("PATCH "+APIVersion+"/combos/{id}", gateway(http.HandlerFunc(deps.Combo.Update)))
	mux.Handle("DELETE "+APIVersion+"/combos/{id}", gateway(http.HandlerFunc(deps.Combo.Delete)))
	mux.Handle("POST "+APIVersion+"/combos/{id}/test", gateway(http.HandlerFunc(deps.ComboTest.Test)))
	mux.Handle("GET "+APIVersion+"/vision-adapter", gateway(http.HandlerFunc(deps.VisionAdapter.Get)))
	mux.Handle("PUT "+APIVersion+"/vision-adapter", gateway(http.HandlerFunc(deps.VisionAdapter.Put)))
	mux.Handle("GET "+APIVersion+"/token-saver", gateway(http.HandlerFunc(deps.TokenSaver.Get)))
	mux.Handle("PUT "+APIVersion+"/token-saver", gateway(http.HandlerFunc(deps.TokenSaver.Put)))

	// §7.11 Proxy pools, session-gated like the rest of management. The
	// candidate route addresses no stored resource, which is why it is
	// /proxies/test rather than /proxies/{id}/test.
	mux.Handle("GET "+APIVersion+"/proxies", gateway(http.HandlerFunc(deps.Proxy.List)))
	mux.Handle("POST "+APIVersion+"/proxies", gateway(http.HandlerFunc(deps.Proxy.Create)))
	mux.Handle("POST "+APIVersion+"/proxies/test", gateway(http.HandlerFunc(deps.Proxy.TestCandidate)))
	mux.Handle("PATCH "+APIVersion+"/proxies/{id}", gateway(http.HandlerFunc(deps.Proxy.Update)))
	mux.Handle("DELETE "+APIVersion+"/proxies/{id}", gateway(http.HandlerFunc(deps.Proxy.Delete)))
	mux.Handle("POST "+APIVersion+"/proxies/{id}/test", gateway(http.HandlerFunc(deps.Proxy.Test)))

	// §7.10 Media providers: three management routes and the six data-plane
	// media routes, registered together in router_media.go.
	registerMediaRoutes(mux, deps, gateway)

	// §7.12–§7.14 Usage, quotas, logs, and settings are management routes, so
	// they share the same session guard.
	mux.Handle("GET "+APIVersion+"/usage/summary", gateway(http.HandlerFunc(deps.Usage.Summary)))
	mux.Handle("GET "+APIVersion+"/usage/timeseries", gateway(http.HandlerFunc(deps.Usage.Timeseries)))
	mux.Handle("GET "+APIVersion+"/usage/records", gateway(http.HandlerFunc(deps.Usage.Records)))
	mux.Handle("GET "+APIVersion+"/usage/records/{request_id}", gateway(http.HandlerFunc(deps.Usage.Detail)))
	// §7.12's live stream is session-gated like the four reads beside it: it
	// carries the same request identity, so it answers to the same credential.
	mux.Handle("GET "+APIVersion+"/usage/live", gateway(http.HandlerFunc(deps.UsageLive.Stream)))
	mux.Handle("GET "+APIVersion+"/quotas", gateway(http.HandlerFunc(deps.Quota.List)))
	mux.Handle("GET "+APIVersion+"/quotas/{endpoint_id}", gateway(http.HandlerFunc(deps.Quota.Get)))
	mux.Handle("PUT "+APIVersion+"/quotas/{endpoint_id}", gateway(http.HandlerFunc(deps.Quota.PutCap)))
	mux.Handle("GET "+APIVersion+"/logs/requests", gateway(http.HandlerFunc(deps.Log.Requests)))
	mux.Handle("DELETE "+APIVersion+"/logs/requests", gateway(http.HandlerFunc(deps.Log.Purge)))
	mux.Handle("GET "+APIVersion+"/logs/requests/{request_id}", gateway(http.HandlerFunc(deps.Log.Detail)))
	mux.Handle("GET "+APIVersion+"/logs/console", gateway(http.HandlerFunc(deps.Log.Console)))
	mux.Handle("DELETE "+APIVersion+"/logs/console", gateway(http.HandlerFunc(deps.Log.ClearConsole)))
	mux.Handle("GET "+APIVersion+"/settings", gateway(http.HandlerFunc(deps.Settings.Get)))
	mux.Handle("PATCH "+APIVersion+"/settings", gateway(http.HandlerFunc(deps.Settings.Patch)))

	// §7.16–§7.18: the skill catalog, the served contract, and the release
	// notes are management routes over embedded static data, session-gated like
	// the rest. The contract route answers with the document itself rather than
	// the §8 envelope, so a reader can diff it against the spec.
	mux.Handle("GET "+APIVersion+"/skills", gateway(http.HandlerFunc(deps.Skills.List)))
	mux.Handle("GET "+APIVersion+"/openapi.json", gateway(http.HandlerFunc(deps.OpenAPI.Document)))
	mux.Handle("GET "+APIVersion+"/changelog", gateway(http.HandlerFunc(deps.Changelog.List)))

	// §7.15 Data plane: registered together in router_dataplane.go, because
	// these are the routes a CLI tool calls and they are not session-gated.
	registerDataPlaneRoutes(mux, deps)

	// Unknown paths and wrong verbs stay the mux's answer so it can distinguish
	// 404 from 405 (and send Allow on the latter); envelope() then restates
	// either in the §8 shape, so routing errors look like every other error.
	return &Mux{Handler: chain(requestRateLimit(mux, deps.RateLimiter, deps.RateLimitPerMin)), routes: mux.patterns}
}
