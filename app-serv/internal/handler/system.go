// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/system.go
// @for       The system endpoints: health and version (SPEC-API-001 §7.1).
// @uses      internal/domain, internal/schema, internal/service.
// @reason    §7.1 makes liveness report DB and Redis reachability so a probe
//
//	distinguishes "process is up" from "dependencies are healthy";
//	AGENTS.md §1.5 keeps net/http out of the service layer, so the
//	HTTP mapping (including a 503 when degraded) lives here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     handler
// @stability experimental
// @since     2026-09-16
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// SystemHandler serves the public system endpoints (§7.1).
type SystemHandler struct {
	info   schema.SystemInfo
	health *service.HealthService
}

// SystemHandlerDeps holds the build info and health service the handler needs.
type SystemHandlerDeps struct {
	Info   schema.SystemInfo
	Health *service.HealthService
}

// NewSystemHandler validates deps and returns the handler.
func NewSystemHandler(deps SystemHandlerDeps) *SystemHandler {
	return &SystemHandler{info: deps.Info, health: deps.Health}
}

// Health serves GET /api/v1/health.
//
// A 200 means every dependency answered; a 503 means at least one did not. The
// "checks" object carries per-dependency detail so an operator can tell which
// one degraded without digging into logs.
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	state, results := h.health.Check(r.Context())

	code := http.StatusOK
	if state != domain.HealthUP {
		code = http.StatusServiceUnavailable
	}
	schema.WriteHealth(w, code, state, results, h.info)
}

// Version serves GET /api/v1/version.
func (h *SystemHandler) Version(w http.ResponseWriter, r *http.Request) {
	schema.WriteSystemInfo(w, h.info)
}
