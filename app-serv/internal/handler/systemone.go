// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/systemone.go
// @for       POST /api/v1/systemone, the System One (Jev) decision route.
// @uses      internal/schema, internal/service, net/http.
// @reason    SPEC-API-001 §7.15 serves the route, and the handler follows the
//
//	embeddings route's shape: decode, validate, authenticate, call, write.
//	The answer is forwarded as the upstream wrote it, because a decision
//	answer's vocabulary belongs to the provider (per-question confidences
//	rather than a chat message), so re-encoding it through a typed struct
//	would invent fields the client did not ask for.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-24
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// SystemOneHandler serves POST /api/v1/systemone (§7.15).
type SystemOneHandler struct {
	systemone *service.SystemOneService
	auth      service.GatewayAuthenticator
}

// NewSystemOneHandler validates deps and returns the handler. The chat service is
// reused for authentication, because the §4 rule is one rule: a second
// implementation of it is how two routes start disagreeing about which key is
// valid.
func NewSystemOneHandler(systemone *service.SystemOneService, auth service.GatewayAuthenticator) *SystemOneHandler {
	return &SystemOneHandler{systemone: systemone, auth: auth}
}

// Decide serves POST /api/v1/systemone.
//
// Authentication runs before the body is read, which is the §4 rule every
// data-plane route follows: an unauthenticated caller is refused whatever its
// body looks like, so a malformed payload cannot be used to probe the schema.
func (h *SystemOneHandler) Decide(w http.ResponseWriter, r *http.Request) {
	key, err := h.auth.Authenticate(r.Context(), bearerToken(r))
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	req, err := schema.DecodeSystemOneRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		writeDataPlaneError(w, err)
		return
	}

	// The key's id travels with the call so the usage and log rows it writes
	// name the key that was admitted (SPEC-API-001 §7.12/§7.13).
	answer, err := h.systemone.Decide(r.Context(), req, key.ID())
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(answer)
}
