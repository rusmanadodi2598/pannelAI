// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/embeddings.go
// @for       POST /api/v1/embeddings, the P1 media data plane route.
// @uses      internal/schema, internal/service, net/http.
// @reason    SPEC-API-001 §7.10 lists embeddings as the P1 media route and §7.15
//
//	serves it on the OpenAI wire. The credential placement differs per
//	provider kind (§8.1), but that decision belongs to the service, so
//	this handler only decodes, authenticates, calls, and encodes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-17
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// EmbeddingsHandler serves POST /api/v1/embeddings (§7.10).
type EmbeddingsHandler struct {
	embeddings *service.EmbeddingsService
	auth       service.GatewayAuthenticator
}

// NewEmbeddingsHandler validates deps and returns the handler. The chat service is
// reused for authentication, because the §4 rule is one rule: a second
// implementation of it is how the two routes start disagreeing about which key is
// valid.
func NewEmbeddingsHandler(embeddings *service.EmbeddingsService, auth service.GatewayAuthenticator) *EmbeddingsHandler {
	return &EmbeddingsHandler{embeddings: embeddings, auth: auth}
}

// Embed serves POST /api/v1/embeddings.
func (h *EmbeddingsHandler) Embed(w http.ResponseWriter, r *http.Request) {
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	req, err := schema.DecodeEmbeddingsRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		writeDataPlaneError(w, err)
		return
	}
	// The union decode is what knows whether `input` carried text or tokens, so an
	// absent input is rejected here rather than by the struct-tag rules, which
	// cannot see inside it.
	if req.Input.IsEmpty() {
		writeDataPlaneError(w, dataplane.ValidationError("field Input is required"))
		return
	}

	key, err := h.auth.Authenticate(r.Context(), bearerToken(r))
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}

	// The key's id travels with the call so the usage and log rows it writes
	// name the key that was admitted (SPEC-API-001 §7.12/§7.13).
	response, _, err := h.embeddings.Embed(r.Context(), req, key.ID())
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, response)
}
