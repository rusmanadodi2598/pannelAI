// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_image.go
// @for       The §7.10 image generation route.
// @uses      internal/schema, net/http.
// @reason    The route decodes a JSON body, authenticates, calls, and answers the normalized envelope. The video route's two files live in media_video.go, because a video generation is an async job with its own request and answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-19
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// Images serves POST /api/v1/images/generations.
func (h *MediaHandler) Images(w http.ResponseWriter, r *http.Request) {
	keyID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	req, err := schema.DecodeImageRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		writeDataPlaneError(w, err)
		return
	}
	response, _, err := h.media.GenerateImage(r.Context(), req, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, response)
}
