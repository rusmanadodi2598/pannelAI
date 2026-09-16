// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/gateway_key.go
// @for       The gateway key CRUD endpoints (SPEC-API-001 §7.3).
// @uses      internal/schema, internal/service.
// @reason    §7.3 requires the plaintext to be returned exactly once on create
//
//	and only key_hint thereafter; AGENTS.md §1.5 keeps validation in
//	the schema layer and business rules in the service layer, so this
//	handler only decodes, calls, and encodes.
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

// GatewayKeyHandler serves the /api/v1/gateway-keys routes (§7.3).
type GatewayKeyHandler struct {
	keys *service.GatewayKeyService
}

// NewGatewayKeyHandler validates deps and returns the handler.
func NewGatewayKeyHandler(keys *service.GatewayKeyService) *GatewayKeyHandler {
	return &GatewayKeyHandler{keys: keys}
}

// List serves GET /api/v1/gateway-keys. Responses carry hints only (§4).
func (h *GatewayKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	keys, total, err := h.keys.List(r.Context(), page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.GatewayKeyList{Data: make([]schema.GatewayKeyResponse, 0, len(keys)), Meta: schema.Page{Page: page, PerPage: perPage, Total: total}}
	for _, k := range keys {
		resp.Data = append(resp.Data, toGatewayKeyResponse(k, ""))
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Create serves POST /api/v1/gateway-keys. The full key is returned once.
func (h *GatewayKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.CreateGatewayKeyRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	key, plaintext, err := h.keys.Create(r.Context(), req.Name)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, toGatewayKeyResponse(key, plaintext))
}

// Get serves GET /api/v1/gateway-keys/{id}.
func (h *GatewayKeyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		schema.WriteError(w, domain.NewValidationError("id is required"))
		return
	}
	key, err := h.keys.Get(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toGatewayKeyResponse(key, ""))
}

// Update serves PATCH /api/v1/gateway-keys/{id}.
func (h *GatewayKeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		schema.WriteError(w, domain.NewValidationError("id is required"))
		return
	}
	var req schema.UpdateGatewayKeyRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	key, err := h.keys.Update(r.Context(), id, req.Name, req.Status)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toGatewayKeyResponse(key, ""))
}

// Delete serves DELETE /api/v1/gateway-keys/{id} (soft revocation).
func (h *GatewayKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		schema.WriteError(w, domain.NewValidationError("id is required"))
		return
	}
	if err := h.keys.Revoke(r.Context(), id); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// toGatewayKeyResponse maps an aggregate to its wire shape. plaintext is only
// non-empty on the create response, where the client sees the key exactly once.
func toGatewayKeyResponse(k domain.GatewayKey, plaintext string) schema.GatewayKeyResponse {
	resp := schema.GatewayKeyResponse{
		ID:           k.ID(),
		Name:         k.Name(),
		KeyHint:      k.KeyHint(),
		Status:       string(k.Status()),
		RequestCount: k.RequestCount(),
		CreatedAt:    schema.Timestamp(k.CreatedAt()),
	}
	if lu := k.LastUsedAt(); lu != nil {
		resp.LastUsedAt = ptr(schema.Timestamp(*lu))
	}
	if rv := k.RevokedAt(); rv != nil {
		resp.RevokedAt = ptr(schema.Timestamp(*rv))
	}
	if plaintext != "" {
		resp.PlaintextKey = plaintext
	}
	return resp
}

// ptr returns a pointer to a copy, so the response can distinguish an unset
// timestamp (omitted) from a set one.
func ptr(v string) *string { return &v }
