// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider.go
// @for       The provider registry routes (SPEC-API-001 §7.4).
// @uses      internal/schema, internal/service, net/http.
// @reason    §7.4 serves the registry read-only, and AGENTS.md §1.5 keeps the
//
//	filtering and the store read in the service: this layer decodes a bounded
//	filter, calls, and maps. Routability travels with every row because §8
//	makes it the field a client reads before configuring an account.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-18
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ProviderHandler serves the /api/v1/providers routes (§7.4).
type ProviderHandler struct {
	providers *service.ProviderService
}

// NewProviderHandler validates deps and returns the handler.
func NewProviderHandler(providers *service.ProviderService) *ProviderHandler {
	return &ProviderHandler{providers: providers}
}

// List serves GET /api/v1/providers with the category, routability, and
// free-text filters.
//
// The search term is trimmed at the boundary, so an empty box means "everything"
// rather than "match the empty string", and it is bounded here rather than in
// the service because a limit on external input belongs where the input enters
// (AGENTS.md §1.4).
func (h *ProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	query := r.URL.Query()
	filter := service.ProviderFilter{
		Category:    query.Get("category"),
		Routability: query.Get("routability"),
		Q:           strings.TrimSpace(query.Get("q")),
	}
	if err := schema.ValidateStruct(schema.ProviderListQuery{
		Routability: filter.Routability,
		Q:           filter.Q,
	}); err != nil {
		schema.WriteError(w, err)
		return
	}

	rows, total, err := h.providers.List(r.Context(), filter, page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.ProviderList{
		Data: make([]schema.ProviderResponse, 0, len(rows)),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	}
	for _, row := range rows {
		resp.Data = append(resp.Data, schema.ProviderResponseFrom(row.Entry, schema.ProviderCountsFrom(row.Summary)))
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Get serves GET /api/v1/providers/{provider_id}.
func (h *ProviderHandler) Get(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	row, err := h.providers.Detail(r.Context(), providerID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ProviderDetailFrom(row.Entry, schema.ProviderCountsFrom(row.Summary)))
}

// Models serves GET /api/v1/providers/{provider_id}/models.
//
// The body names where the list came from. A registry provider answers from the
// embedded document; a custom node's list is read from its own upstream, and a
// node whose upstream cannot answer falls back to what the registry holds with a
// warning — never a 5xx, because a node whose upstream is down still routes.
//
// The `?suggested` parameter is gone. §7.4 used to name it and the embedded
// registry never carried a suggestion flag, so it filtered nothing while the
// body answered `suggested: true` for every row; the reference has no such
// parameter either (draft 017 §4.5). `source` is what replaced it.
func (h *ProviderHandler) Models(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	list, err := h.providers.Models(r.Context(), providerID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ProviderModelList{
		Data:    schema.ProviderModelsFrom(list.Entry),
		Source:  list.Source,
		Warning: list.Warning,
	})
}
