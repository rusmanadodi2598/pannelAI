// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/token_count.go
// @for       POST /api/v1/messages/count_tokens, the P3 token estimate route.
// @uses      internal/schema, internal/service, net/http.
// @reason    SPEC-API-001 §7.15 serves the route on the Anthropic wire under the
//
//	same §4 gateway-key rule as the chat routes. It shares that rule
//	through the GatewayAuthenticator seam rather than the chat service,
//	so the route can be proven without the engine the chat service needs.
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

// TokenCountHandler serves POST /api/v1/messages/count_tokens (§7.15).
type TokenCountHandler struct {
	count *service.TokenCountService
	auth  service.GatewayAuthenticator
}

// NewTokenCountHandler validates deps and returns the handler. The chat service
// is reused for authentication in production, because the §4 rule is one rule.
func NewTokenCountHandler(count *service.TokenCountService, auth service.GatewayAuthenticator) *TokenCountHandler {
	return &TokenCountHandler{count: count, auth: auth}
}

// Count serves the route: read the body, decode and validate it against the
// Anthropic contract, authenticate, then answer the estimate.
//
// The route writes no usage or log row: it dials no upstream, so nothing is
// spent, and it is not a refusal of a resolved model (the same boundary the
// models list keeps). The authenticated call still advances the presenting
// key's own counter through the §4 choke point (SPEC-API-001 §7.3).
func (h *TokenCountHandler) Count(w http.ResponseWriter, r *http.Request) {
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	decoded, err := schema.DecodeCountTokensRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := schema.ValidateStruct(decoded); err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if _, err := h.auth.Authenticate(r.Context(), bearerToken(r)); err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, h.count.Count(decoded))
}
