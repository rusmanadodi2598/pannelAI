// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_device.go
// @for       The two device-flow routes: minting a verification round and answering one poll of it (SPEC-API-001 §7.4).
// @uses      internal/schema, internal/service, net/http.
// @reason    A device flow has no browser callback, so the panel drives it by asking: start once, poll until the vendor answers. Both routes are management routes, the operator's own session is what makes the poll belong to the flow that started it, and each one is a decode, a call, and an encode with no policy of its own.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-27
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// DeviceStart serves POST /api/v1/providers/{provider_id}/oauth/device/start.
// The request carries no body: the provider is the path, and everything else the
// round needs the gateway mints itself.
func (h *OAuthHandler) DeviceStart(w http.ResponseWriter, r *http.Request) {
	start, err := h.flow.DeviceStart(r.Context(), service.OAuthDeviceStartInput{
		ProviderID: r.PathValue("provider_id"),
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.OAuthDeviceStartResponse{
		DeviceCode:      start.DeviceCode,
		VerificationURL: start.VerificationURL,
		UserCode:        start.UserCode,
		IntervalSeconds: start.IntervalSeconds,
		ExpiresIn:       start.ExpiresIn,
	})
}

// DevicePoll serves POST /api/v1/providers/{provider_id}/oauth/device/poll. One
// call performs exactly one upstream attempt, so the panel's cadence is the only
// thing that decides how often the vendor is asked.
func (h *OAuthHandler) DevicePoll(w http.ResponseWriter, r *http.Request) {
	var req schema.OAuthDevicePollRequest
	if err := schema.DecodeJSON(r, &req); err != nil {
		schema.WriteError(w, err)
		return
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return
	}
	answer, err := h.flow.DevicePoll(r.Context(), service.OAuthDevicePollInput{
		ProviderID: r.PathValue("provider_id"),
		DeviceCode: req.DeviceCode,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.OAuthDevicePollResponse{
		Status:     answer.Status,
		EndpointID: answer.EndpointID,
		TokenHint:  answer.TokenHint,
		Created:    answer.Created,
	})
}
