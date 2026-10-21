// Package router wires the app-bot HTTP surface.
//
// @file      internal/router/router.go
// @for       The route table and the handler chain order.
// @uses      net/http, handler, logx.
// @reason    Every route answers through one middleware chain, so request id, panic recovery, and access logging cannot be forgotten on a new route.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability stable
// @since     2026-10-21
package router

import (
	"log/slog"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/handler"
)

// Deps is the composition root's contribution to the router.
type Deps struct {
	System *handler.System
	Logger *slog.Logger
}

// New builds the route table. Order of wrapping is deliberate: recovery is
// outermost so a panic in any middleware still answers, then the request id,
// then access logging.
func New(deps Deps) http.Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", deps.System.Health)
	mux.HandleFunc("GET /readyz", deps.System.Ready)
	mux.Handle("GET /metrics", loopbackOnly(http.HandlerFunc(deps.System.Metrics)))

	chain := recoverPanics(deps.Logger, accessLog(deps.Logger, requestID(mux)))
	return chain
}
