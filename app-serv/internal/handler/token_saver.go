// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/token_saver.go
// @for       The token-saver read and whole-replacement routes (SPEC-API-001
//
//	§7.9).
//
// @uses      internal/schema, internal/service, net/http.
// @reason    §7.9 defines one shape for the read and the write so the panel
//
//	can PUT back exactly what it GET. The handler keeps that symmetry:
//	it decodes, calls the service, and encodes, and the deprecated
//	caveman key has no route here by design (§7.9).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// TokenSaverHandler serves the /api/v1/token-saver routes (§7.9).
type TokenSaverHandler struct {
	saver *service.TokenSaverService
}

// NewTokenSaverHandler validates deps and returns the handler.
func NewTokenSaverHandler(saver *service.TokenSaverService) *TokenSaverHandler {
	return &TokenSaverHandler{saver: saver}
}

// Get serves GET /api/v1/token-saver: the §7.9 configuration with the stored
// values merged over the documented defaults.
func (h *TokenSaverHandler) Get(w http.ResponseWriter, r *http.Request) {
	saver, err := h.saver.Get(r.Context())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToTokenSaverResponse(saver))
}

// Put serves PUT /api/v1/token-saver, replacing the configuration. Every group
// is required, so a partial body is a client mistake the schema rejects before
// the service sees it.
func (h *TokenSaverHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req schema.ReplaceTokenSaverRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	saver, err := h.saver.Replace(r.Context(), req.ToTokenSaverSettings())
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.ToTokenSaverResponse(saver))
}
