// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/proxy.go
// @for       The proxy pool endpoints (SPEC-API-001 §7.11).
// @uses      internal/schema, internal/service, internal/domain, net/http.
// @reason    §7.11 fixes six routes whose only work is decode → validate → call
//
//	→ encode. The password is write-only, so no response this file writes
//	carries it; a failed connectivity test is a 200 with a fail state, not
//	an error, because that is the answer the operator asked for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ProxyHandler serves the /api/v1/proxies routes (§7.11).
type ProxyHandler struct {
	proxies *service.ProxyService
}

// NewProxyHandler validates deps and returns the handler.
func NewProxyHandler(proxies *service.ProxyService) *ProxyHandler {
	return &ProxyHandler{proxies: proxies}
}

// List serves GET /api/v1/proxies. §7.11 lists the pool without pagination, so
// the whole set is returned in one page.
func (h *ProxyHandler) List(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.proxies.List(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ProxyList{Data: schema.ToProxyResponses(proxies)})
}

// Create serves POST /api/v1/proxies.
func (h *ProxyHandler) Create(w http.ResponseWriter, r *http.Request) {
	draft, err := decodeProxyRequest(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	proxy, err := h.proxies.Create(r.Context(), draft)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusCreated, schema.ToProxyResponse(proxy))
}

// Update serves PATCH /api/v1/proxies/{id}. An empty password keeps the stored
// secret; the service decides that, not this layer.
func (h *ProxyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	draft, err := decodeProxyRequest(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	proxy, err := h.proxies.Update(r.Context(), id, draft)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToProxyResponse(proxy))
}

// Delete serves DELETE /api/v1/proxies/{id}.
func (h *ProxyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	if err := h.proxies.Delete(r.Context(), id); err != nil {
		schema.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test serves POST /api/v1/proxies/{id}/test: it probes the stored candidate,
// stores what it found, and answers with the same status the list will report.
func (h *ProxyHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := pathValue(w, r, "id")
	if !ok {
		return
	}
	status, err := h.proxies.Test(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toProxyTestResponse(status))
}

// TestCandidate serves POST /api/v1/proxies/test: it probes an unsaved
// candidate and stores nothing, which is what the panel uses while the
// operator is still filling the form.
func (h *ProxyHandler) TestCandidate(w http.ResponseWriter, r *http.Request) {
	target, err := decodeProxyCandidate(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	status, err := h.proxies.TestCandidate(r.Context(), target)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toProxyTestResponse(status))
}

// decodeProxyRequest decodes and validates a create or patch body and converts
// it into the service input. Both routes carry the same shape, so they share
// the decode path too and cannot diverge.
func decodeProxyRequest(r *http.Request) (service.ProxyDraft, error) {
	var req schema.ProxyRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		return service.ProxyDraft{}, err
	}
	if err := schema.ValidateStruct(req); err != nil {
		return service.ProxyDraft{}, err
	}
	protocol, err := domain.ParseProxyProtocol(req.Protocol)
	if err != nil {
		return service.ProxyDraft{}, err
	}
	return service.ProxyDraft{
		Label:    req.Label,
		Protocol: protocol,
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Enabled:  req.Enabled,
	}, nil
}

// decodeProxyCandidate decodes and validates the unsaved-candidate body.
func decodeProxyCandidate(r *http.Request) (service.ProxyTarget, error) {
	var req schema.ProxyCandidateRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		return service.ProxyTarget{}, err
	}
	if err := schema.ValidateStruct(req); err != nil {
		return service.ProxyTarget{}, err
	}
	protocol, err := domain.ParseProxyProtocol(req.Protocol)
	if err != nil {
		return service.ProxyTarget{}, err
	}
	return service.ProxyTarget{
		Protocol: protocol,
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}, nil
}

// toProxyTestResponse maps a test result onto the wire shape both test routes
// answer with.
func toProxyTestResponse(status domain.ProxyTestStatus) schema.ProxyTestResponse {
	checkedAt := ""
	if status.CheckedAt != nil {
		checkedAt = schema.Timestamp(*status.CheckedAt)
	}
	return schema.ProxyTestResponse{
		State:     status.State,
		LatencyMS: status.LatencyMS,
		CheckedAt: checkedAt,
		Message:   status.Message,
	}
}
