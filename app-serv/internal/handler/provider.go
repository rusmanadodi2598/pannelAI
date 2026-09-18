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

// List serves GET /api/v1/providers with the category and routability filters.
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
	}
	if err := schema.ValidateStruct(struct {
		Routability string `validate:"omitempty,oneof=native connector"`
	}{Routability: filter.Routability}); err != nil {
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
// The suggested filter is accepted because §7.4 names it, and it is a no-op
// here: the embedded registry has no suggestion flag, so every model the
// document lists is offered. Silently ignoring the parameter would be worse than
// this comment, which is why the behaviour is stated rather than implied.
func (h *ProviderHandler) Models(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	entry, err := h.providers.Models(r.Context(), providerID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ProviderModelList{Data: schema.ProviderModelsFrom(entry)})
}
