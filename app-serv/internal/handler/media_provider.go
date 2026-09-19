// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_provider.go
// @for       The media provider endpoints (SPEC-API-001 §7.10).
// @uses      internal/schema, internal/service, internal/domain, internal/registry,
//
//	net/http, strings.
//
// @reason    §7.10 fixes three routes whose only work is decode → validate →
//
//	call → encode. The kind filter is parsed here rather than passed through
//	so an unknown kind is a 400 naming the closed set instead of an empty
//	page the panel would render as "nothing configured". The view → wire
//	mapping lives here too: schema may not import service (it would cycle
//	back through dataplane), so the layer that already holds both maps it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// MediaProviderHandler serves the /api/v1/media-providers routes (§7.10).
type MediaProviderHandler struct {
	media *service.MediaProviderService
}

// NewMediaProviderHandler validates deps and returns the handler.
func NewMediaProviderHandler(media *service.MediaProviderService) *MediaProviderHandler {
	return &MediaProviderHandler{media: media}
}

// List serves GET /api/v1/media-providers?kind=. An absent kind lists every
// media kind the registry offers.
func (h *MediaProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	kind, err := mediaQueryKind(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	views, err := h.media.List(r.Context(), kind)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.MediaProviderList{Data: toMediaProviderResponses(views)})
}

// Get serves GET /api/v1/media-providers/{provider_id}.
func (h *MediaProviderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	views, err := h.media.Detail(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toMediaProviderDetail(views))
}

// Patch serves PATCH /api/v1/media-providers/{provider_id}: it saves one kind's
// override and answers with the resolved block, so the panel re-renders what
// the gateway will actually dial.
func (h *MediaProviderHandler) Patch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	draft, err := decodeMediaOverrideRequest(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	view, err := h.media.Patch(r.Context(), id, draft)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toMediaKindBlock(view))
}

// toMediaProviderResponses maps the list views onto the wire shape.
func toMediaProviderResponses(views []service.MediaServiceView) []schema.MediaProviderResponse {
	out := make([]schema.MediaProviderResponse, 0, len(views))
	for _, view := range views {
		out = append(out, schema.MediaProviderResponse{
			ProviderID:     view.ProviderID,
			ProviderName:   view.ProviderName,
			MediaKindBlock: toMediaKindBlock(view),
		})
	}
	return out
}

// toMediaProviderDetail maps the detail views onto the wire shape.
func toMediaProviderDetail(views []service.MediaServiceView) schema.MediaProviderDetail {
	detail := schema.MediaProviderDetail{Media: toMediaKindBlocks(views)}
	if len(views) > 0 {
		detail.ProviderID = views[0].ProviderID
		detail.ProviderName = views[0].ProviderName
	}
	return detail
}

// toMediaKindBlock maps one view onto the wire shape.
func toMediaKindBlock(view service.MediaServiceView) schema.MediaKindBlock {
	return schema.MediaKindBlock{
		Kind:               string(view.Kind),
		BaseURL:            view.BaseURL,
		BaseURLSource:      view.BaseURLSource,
		DefaultModel:       view.DefaultModel,
		DefaultModelSource: view.DefaultModelSource,
		EndpointCount:      view.EndpointCount,
		Models:             toMediaModels(view.Models),
	}
}

// toMediaKindBlocks maps a detail's kind list.
func toMediaKindBlocks(views []service.MediaServiceView) []schema.MediaKindBlock {
	out := make([]schema.MediaKindBlock, 0, len(views))
	for _, view := range views {
		out = append(out, toMediaKindBlock(view))
	}
	return out
}

// toMediaModels maps the registry's model list, which is empty for a kind that
// declares none.
func toMediaModels(models []registry.MediaModel) []schema.MediaModelResponse {
	out := make([]schema.MediaModelResponse, 0, len(models))
	for _, model := range models {
		out = append(out, schema.MediaModelResponse{ID: model.ID, Name: model.Name, Dimensions: model.Dimensions})
	}
	return out
}

// mediaQueryKind reads the optional ?kind= filter.
func mediaQueryKind(r *http.Request) (domain.MediaKind, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("kind"))
	if raw == "" {
		return "", nil
	}
	return domain.ParseMediaKind(raw)
}

// decodeMediaOverrideRequest decodes and validates the save body and converts
// it into the service input.
func decodeMediaOverrideRequest(r *http.Request) (service.MediaOverrideDraft, error) {
	var req schema.MediaOverrideRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		return service.MediaOverrideDraft{}, err
	}
	if err := schema.ValidateStruct(req); err != nil {
		return service.MediaOverrideDraft{}, err
	}
	kind, err := domain.ParseMediaKind(req.Kind)
	if err != nil {
		return service.MediaOverrideDraft{}, err
	}
	return service.MediaOverrideDraft{
		Kind:         kind,
		BaseURL:      req.BaseURL,
		DefaultModel: req.DefaultModel,
	}, nil
}
