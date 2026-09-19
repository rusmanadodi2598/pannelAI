// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_search.go
// @for       The §7.10 web search route.
// @uses      internal/schema, net/http.
// @reason    The route decodes, authenticates, calls, and answers the normalized
//
//	envelope; the provider and parameter decisions belong to the service,
//	so nothing provider-specific reaches this layer.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// Search serves POST /api/v1/search.
func (h *MediaHandler) Search(w http.ResponseWriter, r *http.Request) {
	keyID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	req, err := schema.DecodeSearchRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		writeDataPlaneError(w, err)
		return
	}
	response, _, err := h.media.Search(r.Context(), req, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, response)
}
