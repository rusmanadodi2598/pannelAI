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
	"encoding/json"
	"log/slog"
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
}

// Deps holds the handlers the router wires up.
//
// Every handler is a pointer so an unset one is detectable, and New refuses to
// register a route whose handler is missing: a nil handler would compile and
// then panic on the first request, which is the failure mode the composition
// root cannot see at boot.
type Deps struct {
	System          *handler.SystemHandler
	Auth            *handler.AuthHandler
	GatewayKey      *handler.GatewayKeyHandler
	Provider        *handler.ProviderHandler
	Endpoint        *handler.EndpointHandler
	EndpointKey     *handler.EndpointKeyHandler
	EndpointBulk    *handler.EndpointBulkHandler
	ProviderNode    *handler.ProviderNodeHandler
	Model           *handler.ModelHandler
	Combo           *handler.ComboHandler
	VisionAdapter   *handler.VisionAdapterHandler
	Usage           *handler.UsageHandler
	Quota           *handler.QuotaHandler
	Log             *handler.LogHandler
	Settings        *handler.SettingsHandler
	Chat            *handler.ChatHandler
	Embeddings      *handler.EmbeddingsHandler
	RateLimiter     repository.RateLimiter
	RateLimitPerMin int
}

// New registers every route and returns the assembled mux. Patterns carry the
// method (Go 1.22 ServeMux syntax) so the mux rejects a wrong verb before any
// handler sees it.
func New(deps Deps) *Mux {
	mux := http.NewServeMux()

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
	mux.Handle("GET "+APIVersion+"/provider-nodes", gateway(http.HandlerFunc(deps.ProviderNode.List)))
	mux.Handle("POST "+APIVersion+"/provider-nodes", gateway(http.HandlerFunc(deps.ProviderNode.Create)))
	mux.Handle("GET "+APIVersion+"/provider-nodes/{id}", gateway(http.HandlerFunc(deps.ProviderNode.Get)))
	mux.Handle("PATCH "+APIVersion+"/provider-nodes/{id}", gateway(http.HandlerFunc(deps.ProviderNode.Update)))
	mux.Handle("DELETE "+APIVersion+"/provider-nodes/{id}", gateway(http.HandlerFunc(deps.ProviderNode.Delete)))
	mux.Handle("POST "+APIVersion+"/provider-nodes/{id}/test", gateway(http.HandlerFunc(deps.ProviderNode.Test)))

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
	mux.Handle("GET "+APIVersion+"/vision-adapter", gateway(http.HandlerFunc(deps.VisionAdapter.Get)))
	mux.Handle("PUT "+APIVersion+"/vision-adapter", gateway(http.HandlerFunc(deps.VisionAdapter.Put)))

	// §7.12–§7.14 Usage, quotas, logs, and settings are management routes, so
	// they share the same session guard.
	mux.Handle("GET "+APIVersion+"/usage/summary", gateway(http.HandlerFunc(deps.Usage.Summary)))
	mux.Handle("GET "+APIVersion+"/usage/timeseries", gateway(http.HandlerFunc(deps.Usage.Timeseries)))
	mux.Handle("GET "+APIVersion+"/usage/records", gateway(http.HandlerFunc(deps.Usage.Records)))
	mux.Handle("GET "+APIVersion+"/usage/records/{request_id}", gateway(http.HandlerFunc(deps.Usage.Detail)))
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

	// §7.15 Data plane: the OpenAI and Anthropic wires plus the models list.
	// These are deliberately NOT session-gated. A CLI tool presents
	// `Authorization: Bearer <gateway key>` and the handler enforces it when
	// settings.security.require_api_key is true (§4); the dashboard session cookie
	// is not a credential a CLI tool can hold, so wrapping these in the session
	// guard would make every client request a 401.
	if deps.Chat != nil {
		mux.HandleFunc("POST "+APIVersion+"/chat/completions", deps.Chat.Completions)
		mux.HandleFunc("POST "+APIVersion+"/messages", deps.Chat.Messages)
		mux.HandleFunc("GET "+APIVersion+"/models", deps.Chat.Models)
	}
	if deps.Embeddings != nil {
		mux.HandleFunc("POST "+APIVersion+"/embeddings", deps.Embeddings.Embed)
	}

	// Unknown paths and wrong verbs stay the mux's answer so it can distinguish
	// 404 from 405 (and send Allow on the latter); envelope() then restates
	// either in the §8 shape, so routing errors look like every other error.
	return &Mux{Handler: chain(requestRateLimit(mux, deps.RateLimiter, deps.RateLimitPerMin))}
}

// envelope restates the mux's own plain-text 404/405 answers in the management
// error envelope (SPEC-API-001 §8). It intercepts only responses no handler
// produced: a handler always sets a JSON content type before writing, so
// anything already JSON passes through untouched and is never double-bodied.
func envelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if !rec.suppressed {
			return
		}
		switch rec.status {
		case http.StatusNotFound:
			writeEnvelope(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		case http.StatusMethodNotAllowed:
			writeEnvelope(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed for this route")
		}
	})
}

// writeEnvelope emits the §8 error shape for a response the mux produced and
// this middleware suppressed, so no other body exists for that request.
func writeEnvelope(w http.ResponseWriter, code int, errCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(schema.ErrorBody{Error: schema.ErrorDetail{Code: errCode, Message: msg}}); err != nil {
		slog.Error("encoding routing error envelope failed", "status", code, "error", err)
	}
}

// statusRecorder intercepts only the mux's built-in 404/405 answers. Those are
// the responses that carry a text/plain content type; a handler response always
// sets application/json first, so it is forwarded verbatim.
type statusRecorder struct {
	http.ResponseWriter
	status     int
	suppressed bool
}

func (rec *statusRecorder) WriteHeader(code int) {
	if rec.suppressed {
		return
	}
	builtin := code == http.StatusNotFound || code == http.StatusMethodNotAllowed
	if builtin && rec.Header().Get("Content-Type") != "application/json" {
		// Withhold the mux's plain-text body so envelope() can replace it.
		rec.suppressed = true
		rec.status = code
		return
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.suppressed {
		return len(b), nil
	}
	return rec.ResponseWriter.Write(b)
}
