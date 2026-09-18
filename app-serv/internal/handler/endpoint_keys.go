// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_keys.go
// @for       The upstream key routes: list, add, patch, delete, and the key batch
//
//	(SPEC-API-001 §7.5).
//
// @uses      internal/domain, internal/schema, internal/service, net/http, time.
// @reason    §7.5 makes a key a child collection of an endpoint, so every route is
//
//	scoped by the endpoint id and a value is write-only: a response
//	carries key_hint and health, never the credential. AGENTS.md §1.5 keeps
//	the sealing and the "keep one active key" rule in the service, so this
//	layer only decodes, calls, and maps.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// EndpointKeyHandler serves the key routes under /api/v1/endpoints/{id}/keys
// (§7.5). It is a separate handler type because the router registers the key
// surface apart from the endpoint surface, and one type holding both would make the
// router's Deps field carry an unused half.
type EndpointKeyHandler struct {
	endpoints *service.EndpointService
}

// NewEndpointKeyHandler validates deps and returns the handler.
func NewEndpointKeyHandler(endpoints *service.EndpointService) *EndpointKeyHandler {
	return &EndpointKeyHandler{endpoints: endpoints}
}

// ListKeys serves GET /api/v1/endpoints/{id}/keys, hints and health only.
func (h *EndpointKeyHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	keys, total, err := h.endpoints.ListKeys(r.Context(), id, page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.EndpointKeyList{
		Data: toKeyResponses(keys, time.Now()),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	})
}

// AddKey serves POST /api/v1/endpoints/{id}/keys.
func (h *EndpointKeyHandler) AddKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	var req schema.AddEndpointKeyRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	key, err := h.endpoints.AddKey(r.Context(), id, service.KeyInput{
		Label: req.Label, Value: req.Value, Priority: req.Priority,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, toKeyResponse(key, time.Now()))
}

// UpdateKey serves PATCH /api/v1/endpoints/{id}/keys/{key_id}. An omitted value
// keeps the stored credential, because the value is write-only on the wire.
func (h *EndpointKeyHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	endpointID, keyID, ok := keyPath(w, r)
	if !ok {
		return
	}
	var req schema.UpdateEndpointKeyRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	key, err := h.endpoints.UpdateKey(r.Context(), endpointID, keyID, service.KeyPatch{
		Label:    req.Label,
		Value:    req.Value,
		Priority: req.Priority,
		Status:   req.Status,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toKeyResponse(key, time.Now()))
}

// DeleteKey serves DELETE /api/v1/endpoints/{id}/keys/{key_id}. The endpoint's own
// rule refuses a removal that would leave an api_key endpoint unable to route, and
// that refusal surfaces as the §8 CONFLICT the panel anticipates (§7.5).
func (h *EndpointKeyHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	endpointID, keyID, ok := keyPath(w, r)
	if !ok {
		return
	}
	if err := h.endpoints.RemoveKey(r.Context(), endpointID, keyID); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// keyPath reads the endpoint id and the key id a key route addresses.
func keyPath(w http.ResponseWriter, r *http.Request) (endpointID, keyID string, ok bool) {
	endpointID, ok = pathValue(w, r, "id")
	if !ok {
		return "", "", false
	}
	keyID, ok = pathValue(w, r, "key_id")
	if !ok {
		return "", "", false
	}
	return endpointID, keyID, true
}
