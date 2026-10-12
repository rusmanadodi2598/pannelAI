// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_video.go
// @for       The §7.10 async video job routes: the creation and the poll.
// @uses      internal/dataplane, internal/schema, net/http, strings.
// @reason    A video generation is an async job, so the surface is two routes rather than one: the creation answers the job, and the poll reads it by id. Both decode, authenticate, call, and answer the normalized envelope, so they share this file rather than one file per verb.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-12
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// VideoEndpointHeader names the endpoint a job was created on. The header
// travels back on the creation so the client can pin its polls to the account
// that owns the job, which is the reference's `x-9router-connection-id`
// (videoGeneration.js:97-104).
const VideoEndpointHeader = "X-PannelAI-Endpoint-Id"

// Videos serves POST /api/v1/videos/generations: it creates one async job and
// answers the normalized job envelope.
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
	job, outcome, err := h.media.GenerateVideoJob(r.Context(), req, raw, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	if outcome.EndpointID != "" {
		w.Header().Set(VideoEndpointHeader, outcome.EndpointID)
	}
	schema.WriteJSON(w, http.StatusOK, job)
}

// VideoJob serves GET /api/v1/videos/{task_id}: it reads one job's state.
//
// The provider is required because a poll carries no model to resolve it from,
// and the endpoint is optional: without one the provider's own selection
// answers, with one the account that created the job does.
func (h *MediaHandler) VideoJob(w http.ResponseWriter, r *http.Request) {
	keyID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	providerID := strings.TrimSpace(r.URL.Query().Get("provider"))
	if providerID == "" {
		writeDataPlaneError(w, dataplane.ValidationError("query parameter provider is required"))
		return
	}
	taskID := strings.TrimSpace(r.PathValue("task_id"))
	if taskID == "" {
		writeDataPlaneError(w, dataplane.ValidationError("a video task id is required"))
		return
	}
	job, _, err := h.media.PollVideoJob(r.Context(), providerID,
		strings.TrimSpace(r.URL.Query().Get("endpoint")), taskID, keyID)
	if err != nil {
		writeDataPlaneError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, job)
}
