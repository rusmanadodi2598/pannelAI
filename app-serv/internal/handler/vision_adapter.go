// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/vision_adapter.go
// @for       The vision adapter endpoints (SPEC-API-001 §7.8).
// @uses      internal/schema, internal/service, net/http.
// @reason    §7.8 defines a GET and a PUT over one configuration, and the PUT is
//
//	a whole replacement rather than a merge; the handler's only job is
//	to keep that symmetry, so what a client GETs is exactly what it may
//	PUT back.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// VisionAdapterHandler serves the /api/v1/vision-adapter routes (§7.8).
type VisionAdapterHandler struct {
	adapter *service.VisionAdapterService
}

// NewVisionAdapterHandler validates deps and returns the handler.
func NewVisionAdapterHandler(adapter *service.VisionAdapterService) *VisionAdapterHandler {
	return &VisionAdapterHandler{adapter: adapter}
}

// Get serves GET /api/v1/vision-adapter.
func (h *VisionAdapterHandler) Get(w http.ResponseWriter, r *http.Request) {
	adapter, err := h.adapter.Get(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToVisionAdapterResponse(adapter))
}

// Put serves PUT /api/v1/vision-adapter, replacing the configuration.
func (h *VisionAdapterHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req schema.ReplaceVisionAdapterRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	refs, err := schema.ToVisionRefs(req.Models)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	adapter, err := h.adapter.Replace(r.Context(), req.Enabled, req.RoundRobin, refs)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToVisionAdapterResponse(adapter))
}
