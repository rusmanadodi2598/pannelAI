// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_node.go
// @for       The custom provider node routes (SPEC-API-001 §7.4).
// @uses      internal/domain, internal/schema, internal/service, net/http.
// @reason    §7.4 fixes the node contract and AGENTS.md §1.5 keeps the prefix
//
//	collision and reference checks in the service: this layer decodes a
//	validated DTO, calls, and maps. A node's prefix is a model-string
//	namespace, so its refusal has to be the §8 CONFLICT a client can act
//	on rather than a driver or registry message.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ProviderNodeHandler serves the /api/v1/provider-nodes routes (§7.4).
type ProviderNodeHandler struct {
	nodes *service.NodeService
}

// NewProviderNodeHandler validates deps and returns the handler.
func NewProviderNodeHandler(nodes *service.NodeService) *ProviderNodeHandler {
	return &ProviderNodeHandler{nodes: nodes}
}

// List serves GET /api/v1/provider-nodes with the optional type filter.
func (h *ProviderNodeHandler) List(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.nodes.List(r.Context(), r.URL.Query().Get("type"))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.ProviderNodeList{Data: make([]schema.ProviderNodeResponse, 0, len(nodes))}
	for _, node := range nodes {
		resp.Data = append(resp.Data, toNodeResponse(node))
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// Create serves POST /api/v1/provider-nodes.
func (h *ProviderNodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.CreateProviderNodeRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	node, err := h.nodes.Create(r.Context(), service.CreateNodeInput{
		Name:    req.Name,
		Prefix:  req.Prefix,
		Type:    domain.NodeType(req.Type),
		APIType: req.APIType,
		BaseURL: req.BaseURL,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, toNodeResponse(node))
}

// Get serves GET /api/v1/provider-nodes/{id}.
func (h *ProviderNodeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	node, err := h.nodes.Get(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// Update serves PATCH /api/v1/provider-nodes/{id}; a prefix that would collide with
// a registry identifier or another node is refused as CONFLICT (§7.4).
func (h *ProviderNodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	var req schema.UpdateProviderNodeRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	node, err := h.nodes.Update(r.Context(), id, service.NodePatch{
		Name:    req.Name,
		Prefix:  req.Prefix,
		BaseURL: req.BaseURL,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// Delete serves DELETE /api/v1/provider-nodes/{id}, refused while an endpoint still
// references the node (CONFLICT, §7.4).
func (h *ProviderNodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	if err := h.nodes.Delete(r.Context(), id); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test serves POST /api/v1/provider-nodes/{id}/test. The body may be absent: a node
// carries no credential of its own, so an operator may test one whose upstream needs
// none.
func (h *ProviderNodeHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeOptionalNodeTest(w, r)
	if !ok {
		return
	}
	outcome, err := h.nodes.TestNode(r.Context(), id, req.Credential)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toProbeResponse(outcome))
}

// toNodeResponse maps a node onto its wire shape. Format is reported because it is
// what decides the path a request to this node takes.
func toNodeResponse(node domain.ProviderNode) schema.ProviderNodeResponse {
	return schema.ProviderNodeResponse{
		ID:        node.ID(),
		Type:      string(node.Type()),
		Name:      node.Name(),
		Prefix:    node.Prefix(),
		APIType:   node.APIType(),
		BaseURL:   node.BaseURL(),
		Format:    node.Format(),
		CreatedAt: schema.Timestamp(node.CreatedAt()),
		UpdatedAt: schema.Timestamp(node.UpdatedAt()),
	}
}
