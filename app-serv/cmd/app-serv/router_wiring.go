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

// routerDeps assembles the router's dependency set from the graph the boot
// sequence already built.
func routerDeps(
	cfg config.Config,
	auth *handler.AuthHandler,
	keys *handler.GatewayKeyHandler,
	health *service.HealthService,
	limiter repository.RateLimiter,
	mgmt managementDeps,
) router.Deps {
	return router.Deps{
		System: handler.NewSystemHandler(handler.SystemHandlerDeps{
			Info:   buildInfo(),
			Health: health,
		}),
		Auth:            auth,
		GatewayKey:      keys,
		Provider:        mgmt.Provider,
		ProviderNode:    mgmt.Node,
		Endpoint:        mgmt.Endpoint,
		EndpointKey:     mgmt.EndpointKey,
		EndpointBulk:    mgmt.EndpointBulk,
		Model:           mgmt.Model,
		Combo:           mgmt.Combo,
		VisionAdapter:   mgmt.VisionAdapter,
		Usage:           mgmt.Usage,
		Quota:           mgmt.Quota,
		Log:             mgmt.Log,
		Settings:        mgmt.Settings,
		Chat:            mgmt.Chat,
		Embeddings:      mgmt.Embeddings,
		RateLimiter:     limiter,
		RateLimitPerMin: cfg.RateLimitPerMin,
	}
}
