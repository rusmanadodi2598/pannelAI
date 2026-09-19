// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_image.go
// @for       The §7.10 image and video generation routes.
// @uses      internal/schema, net/http.
// @reason    Both routes decode a JSON body, authenticate, call, and answer the
//
//	normalized envelope, so they share one request path and differ only in
//	the contract they decode into.
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

// Videos serves POST /api/v1/videos/generations.
func (h *MediaHandler) Videos(w http.ResponseWriter, r *http.Request) {
	keyID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	raw, err := schema.ReadBody(r)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	req, err := schema.DecodeVideoRequest(raw)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		writeDataPlaneError(w, err)
		return
	}
	response, _, err := h.media.GenerateVideo(r.Context(), req, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, response)
}
