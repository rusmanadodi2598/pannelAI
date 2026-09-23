// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_validate.go
// @for       The §7.4 stateless credential-check routes.
// @uses      internal/schema, internal/service, net/http.
// @reason    Draft 017 §4.6 adds two routes that prove a credential before a row
//
//	exists. They live beside the node routes rather than in a new file per
//	route because they share one shape: decode a validated DTO, call, map.
//	The handler holds no store, which is what makes the stateless property
//	visible from the HTTP layer as well as the service.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-23
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ProviderValidateHandler serves the two §7.4 validate routes.
type ProviderValidateHandler struct {
	validation *service.CredentialValidationService
}

// NewProviderValidateHandler validates deps and returns the handler.
func NewProviderValidateHandler(validation *service.CredentialValidationService) *ProviderValidateHandler {
	return &ProviderValidateHandler{validation: validation}
}

// Node serves POST /api/v1/provider-nodes/validate.
//
// It answers 200 with a `state` rather than 4xx for a rejected credential: the
// request was well-formed and the answer is "this credential does not work",
// which is a result, not an error. Only a malformed body is a 400.
func (h *ProviderValidateHandler) Node(w http.ResponseWriter, r *http.Request) {
	var req schema.ValidateNodeRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	outcome, err := h.validation.ValidateNode(r.Context(), service.CredentialCheck{
		BaseURL:    req.BaseURL,
		NodeType:   req.Type,
		APIType:    req.APIType,
		Credential: req.Credential,
		ModelID:    req.ModelID,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, validateResponse(outcome))
}

// Provider serves POST /api/v1/providers/validate.
func (h *ProviderValidateHandler) Provider(w http.ResponseWriter, r *http.Request) {
	var req schema.ValidateProviderRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	outcome, err := h.validation.ValidateProvider(r.Context(), service.CredentialCheck{
		ProviderID: req.ProviderID,
		Credential: req.Credential,
		ModelID:    req.ModelID,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, validateResponse(outcome))
}

// validateResponse maps an outcome onto the wire shape.
//
// It carries `method` as well as `state`, which is the field that makes the two
// probes distinguishable: a client showing "valid" should be able to say which
// request proved it (draft 017 §4.6).
func validateResponse(outcome service.ProbeOutcome) schema.ValidateResponse {
	return schema.ValidateResponse{
		State:     outcome.State,
		Method:    outcome.Method,
		LatencyMS: outcome.LatencyMS,
		Status:    outcome.Status,
		Message:   outcome.Message,
	}
}
