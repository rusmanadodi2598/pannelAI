// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media.go
// @for       The §7.10 data-plane media handler and its shared authentication.
// @uses      internal/schema, internal/service, net/http.
// @reason    §7.10's media routes are one surface with one credential rule:
//
//	they authenticate with a gateway key exactly as the chat and embeddings
//	wires do, so the authentication is written once here and each route
//	file carries only its own decoding and answering.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// MediaHandler serves the §7.10 data-plane media routes.
type MediaHandler struct {
	media *service.MediaCallService
	auth  service.GatewayAuthenticator
}

// NewMediaHandler validates deps and returns the handler. The authenticator is
// the chat service in production because §4's rule is one rule: a second
// implementation is how two routes start disagreeing about which key is valid.
func NewMediaHandler(media *service.MediaCallService, auth service.GatewayAuthenticator) *MediaHandler {
	return &MediaHandler{media: media, auth: auth}
}

// authorize applies the §4 gateway-key rule, writing the OpenAI envelope and
// returning false when the request may not proceed.
//
// The authenticated key's id comes back with the verdict rather than being
// dropped: every call it admits is recorded against that key (SPEC-API-001
// §7.12/§7.13), and the service layer must not have to authenticate a second
// time to learn who is calling.
func (h *MediaHandler) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	key, err := h.auth.Authenticate(r.Context(), bearerToken(r))
	if err != nil {
		writeDataPlaneError(w, err)
		return "", false
	}
	return key.ID(), true
}
