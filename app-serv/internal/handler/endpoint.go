// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint.go
// @for       The upstream endpoint CRUD, test, and batch routes
//
//	(SPEC-API-001 §7.5).
//
// @uses      internal/domain, internal/schema, internal/service, net/http, time.
// @reason    §7.5 fixes the endpoint wire contract and AGENTS.md §1.5 keeps SQL and
//
//	business rules out of this layer: the handler decodes a validated DTO,
//	calls the service, and maps the aggregate onto the response shape. A
//	credential never appears in a response — only the hint the aggregate
//	already holds.
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

// EndpointHandler serves the /api/v1/endpoints routes (§7.5).
type EndpointHandler struct {
	endpoints *service.EndpointService
}

// NewEndpointHandler validates deps and returns the handler.
func NewEndpointHandler(endpoints *service.EndpointService) *EndpointHandler {
	return &EndpointHandler{endpoints: endpoints}
}

// List serves GET /api/v1/endpoints with the provider and status filters.
func (h *EndpointHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := schema.DecodePage(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	query := r.URL.Query()
	endpoints, total, err := h.endpoints.List(r.Context(), query.Get("provider_id"), query.Get("status"), page, perPage)
	if err != nil {
		schema.WriteError(w, err)
		return
	}

	now := time.Now()
	resp := schema.EndpointList{
		Data: make([]schema.EndpointResponse, 0, len(endpoints)),
		Meta: schema.Page{Page: page, PerPage: perPage, Total: total},
	}
	for _, endpoint := range endpoints {
		resp.Data = append(resp.Data, toEndpointResponse(endpoint, now, false))
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Create serves POST /api/v1/endpoints. The stored credential is never returned,
// only its hint.
func (h *EndpointHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.CreateEndpointRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	authType, err := schema.ParseAuthType(req.AuthType)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	endpoint, err := h.endpoints.Create(r.Context(), service.CreateInput{
		ProviderID: req.ProviderID,
		Label:      req.Label,
		AuthType:   authType,
		Priority:   req.Priority,
		Keys:       toKeyInputs(req.Keys),
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, toEndpointResponse(endpoint, time.Now(), true))
}

// Get serves GET /api/v1/endpoints/{id}, including the keys table.
func (h *EndpointHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	endpoint, err := h.endpoints.Get(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toEndpointResponse(endpoint, time.Now(), true))
}

// Update serves PATCH /api/v1/endpoints/{id}.
func (h *EndpointHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	var req schema.UpdateEndpointRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	endpoint, err := h.endpoints.Update(r.Context(), id, service.UpdatePatch{
		Label:          req.Label,
		Priority:       req.Priority,
		Status:         req.Status,
		DefaultModel:   req.DefaultModel,
		GlobalPriority: req.GlobalPriority,
		ProxyPoolID:    req.ProxyPoolID,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toEndpointResponse(endpoint, time.Now(), true))
}

// Delete serves DELETE /api/v1/endpoints/{id}; its keys go with it.
func (h *EndpointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	if err := h.endpoints.Delete(r.Context(), id); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test serves POST /api/v1/endpoints/{id}/test. An optional key_id targets one key;
// otherwise the first active key is used.
//
// A refused credential is a 200 carrying a fail state, not an error: §7.5 makes
// this route answer "does the credential work", and a failed test is a valid answer
// the panel must render rather than an error page.
func (h *EndpointHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeOptionalTestEndpoint(w, r)
	if !ok {
		return
	}
	_, outcome, err := h.endpoints.Test(r.Context(), id, req.KeyID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toProbeResponse(outcome))
}

// toKeyInputs maps decoded key DTOs onto the service's input type, so the handler
// stays the only place that knows both vocabularies.
func toKeyInputs(items []schema.EndpointKeyInput) []service.KeyInput {
	if len(items) == 0 {
		return nil
	}
	out := make([]service.KeyInput, 0, len(items))
	for _, item := range items {
		out = append(out, service.KeyInput{Label: item.Label, Value: item.Value, Priority: item.Priority})
	}
	return out
}
