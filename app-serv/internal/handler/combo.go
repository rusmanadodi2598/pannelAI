// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/combo.go
// @for       The combo CRUD endpoints (SPEC-API-001 §7.7).
// @uses      internal/schema, internal/service, internal/domain, net/http.
// @reason    §7.7 fixes five routes whose only work is decode → validate → call
//
//	→ encode; AGENTS.md §1.5 keeps the strategy rules in the domain
//	layer, so no branch here decides whether a judge model is required.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ComboHandler serves the /api/v1/combos routes (§7.7).
type ComboHandler struct {
	combos *service.ComboService
}

// NewComboHandler validates deps and returns the handler.
func NewComboHandler(combos *service.ComboService) *ComboHandler {
	return &ComboHandler{combos: combos}
}

// List serves GET /api/v1/combos.
func (h *ComboHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	combos, total, err := h.combos.List(r.Context(), page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ComboList{
		Data: schema.ToComboResponses(combos),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	})
}

// Create serves POST /api/v1/combos.
func (h *ComboHandler) Create(w http.ResponseWriter, r *http.Request) {
	draft, err := decodeComboRequest(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	combo, err := h.combos.Create(r.Context(), draft)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, schema.ToComboResponse(combo))
}

// Get serves GET /api/v1/combos/{id}.
func (h *ComboHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := comboPathID(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	combo, err := h.combos.Get(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToComboResponse(combo))
}

// Update serves PATCH /api/v1/combos/{id}.
func (h *ComboHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := comboPathID(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	draft, err := decodeComboRequest(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	combo, err := h.combos.Update(r.Context(), id, draft)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToComboResponse(combo))
}

// Delete serves DELETE /api/v1/combos/{id}. A combo still referenced by an alias
// is refused with CONFLICT (§7.7), which the service decides.
func (h *ComboHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := comboPathID(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := h.combos.Delete(r.Context(), id); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeComboRequest decodes and validates a create or patch body and converts
// it into the service input. Both routes carry the same shape, so they share the
// decode path too and cannot diverge.
func decodeComboRequest(r *http.Request) (service.ComboDraft, error) {
	var req schema.ComboRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		return service.ComboDraft{}, err
	}
	if err := schema.ValidateStruct(req); err != nil {
		return service.ComboDraft{}, err
	}
	strategy, err := domain.ParseComboStrategy(req.Strategy)
	if err != nil {
		return service.ComboDraft{}, err
	}
	models, err := schema.ToComboModels(req.Models)
	if err != nil {
		return service.ComboDraft{}, err
	}
	return service.ComboDraft{
		Name:        req.Name,
		Strategy:    strategy,
		StickyLimit: req.StickyLimit,
		JudgeModel:  req.JudgeModel,
		Models:      models,
	}, nil
}

// comboPathID reads the {id} path value.
func comboPathID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return "", domain.NewValidationError("id is required")
	}
	return id, nil
}
