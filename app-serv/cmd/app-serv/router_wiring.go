// Command app-serv maps the assembled dependency graph onto the router.
//
// @file      cmd/app-serv/router_wiring.go
// @for       The single place the handler set becomes router.Deps.
// @uses      internal/config, internal/handler, internal/repository,
//
//	internal/router, internal/service.
//
// @reason    AGENTS.md §1.5 makes the composition root wiring only, and §1.1
//
//	keeps main.go inside its line budget. Every field of router.Deps is set in
//	one place so a handler the graph builds but the table forgets is visible
//	here rather than as a nil-handler refusal at boot.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package main

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/router"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// managementDeps is the graph the router consumes, plus the workers the caller
// runs after the server is listening. It is declared here rather than beside
// buildManagement because this file is where every field is spent, so a handler
// the graph builds but the table forgets is visible in one screen.
type managementDeps struct {
	Provider      *handler.ProviderHandler
	Endpoint      *handler.EndpointHandler
	EndpointKey   *handler.EndpointKeyHandler
	EndpointBulk  *handler.EndpointBulkHandler
	OAuth         *handler.OAuthHandler
	Node          *handler.ProviderNodeHandler
	Model         *handler.ModelHandler
	Combo         *handler.ComboHandler
	ComboTest     *handler.ComboTestHandler
	Proxy         *handler.ProxyHandler
	MediaProvider *handler.MediaProviderHandler
	Media         *handler.MediaHandler
	VisionAdapter *handler.VisionAdapterHandler
	TokenSaver    *handler.TokenSaverHandler
	Usage         *handler.UsageHandler
	Quota         *handler.QuotaHandler
	Log           *handler.LogHandler
	Settings      *handler.SettingsHandler
	Chat          *handler.ChatHandler
	Embeddings    *handler.EmbeddingsHandler
	TokenCount    *handler.TokenCountHandler

	// QuotaFlusher, LogRetention, and OAuthRefresh are returned so the caller
	// can run them after the server is listening, rather than leaving
	// goroutines nothing supervises.
	QuotaFlusher *service.QuotaFlusher
	LogRetention *service.LogRetentionWorker
	OAuthRefresh *service.OAuthRefreshWorker
}

// routerDeps assembles the router's dependency set from the graph the boot
// sequence already built.
func routerDeps(
	cfg config.Config,
	auth *handler.AuthHandler,
	keys *handler.GatewayKeyHandler,
	health *service.HealthService,
	limiter repository.RateLimiter,
	mgmt managementDeps,
	registryRevision string,
) router.Deps {
	return router.Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   buildInfo(registryRevision),
			Health: health,
		}),
		Auth:            auth,
		GatewayKey:      keys,
		Provider:        mgmt.Provider,
		ProviderNode:    mgmt.Node,
		Endpoint:        mgmt.Endpoint,
		EndpointKey:     mgmt.EndpointKey,
		EndpointBulk:    mgmt.EndpointBulk,
		OAuth:           mgmt.OAuth,
		Model:           mgmt.Model,
		Combo:           mgmt.Combo,
		ComboTest:       mgmt.ComboTest,
		Proxy:           mgmt.Proxy,
		MediaProvider:   mgmt.MediaProvider,
		Media:           mgmt.Media,
		VisionAdapter:   mgmt.VisionAdapter,
		TokenSaver:      mgmt.TokenSaver,
		Usage:           mgmt.Usage,
		Quota:           mgmt.Quota,
		Log:             mgmt.Log,
		Settings:        mgmt.Settings,
		Skills:          handler.NewSkillsHandler(),
		OpenAPI:         handler.NewOpenAPIHandler(),
		Changelog:       handler.NewChangelogHandler(),
		Chat:            mgmt.Chat,
		Embeddings:      mgmt.Embeddings,
		TokenCount:      mgmt.TokenCount,
		RateLimiter:     limiter,
		RateLimitPerMin: cfg.RateLimitPerMin,
	}
}
