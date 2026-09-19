// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth.go
// @for       Starting an authorization, reporting token state, and forcing a
//
//	refresh (SPEC-API-001 §7.4).
//
// @uses      internal/schema, internal/service, net/http, strings.
// @reason    §7.4 splits the four OAuth routes by audience: three are
//
//	session-gated management routes the panel calls, and the callback
//	is public because the provider's browser redirect cannot carry a
//	session cookie. The callback's two-audience answer lives in
//	oauth_callback.go; what stays here is the management half, which
//	only decodes, calls, and encodes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// OAuthHandler serves the §7.4 OAuth routes.
type OAuthHandler struct {
	flow *service.OAuthFlowService
	// baseURL is the public URL the gateway answers on, when one is
	// configured. It is what Start turns into the gateway's own callback URL
	// and what the callback sends a browser back to, so a deployment that
	// sets it never depends on a request's Host header for either.
	baseURL string
}

// NewOAuthHandler validates deps and returns the handler.
func NewOAuthHandler(flow *service.OAuthFlowService, baseURL string) *OAuthHandler {
	return &OAuthHandler{flow: flow, baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/")}
}

// Start serves POST /api/v1/providers/{provider_id}/oauth/start: it returns the
// authorize URL the panel opens and the state that flow is bound to.
func (h *OAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	var req schema.OAuthStartRequest
	if r.ContentLength != 0 {
		if err := schema.DecodeJSON(r, &req); err != nil {
			schema.WriteError(w, err)
			return
		}
		if err := schema.ValidateStruct(req); err != nil {
			schema.WriteError(w, err)
			return
		}
	}
	started, err := h.flow.Start(r.Context(), service.OAuthStartInput{
		ProviderID:  r.PathValue("provider_id"),
		RedirectURI: req.RedirectURI,
		BaseURL:     h.baseURL,
	})
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, schema.OAuthStartResponse{
		AuthorizeURL: started.AuthorizeURL, State: started.State,
	})
}

// Status serves GET /api/v1/providers/{provider_id}/oauth/status: one row per
// OAuth endpoint of the provider, with the expiry and the derived refresh
// state and never the tokens themselves.
func (h *OAuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.flow.Status(r.Context(), r.PathValue("provider_id"))
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, toOAuthStatusResponse(status))
}

// Refresh serves POST /api/v1/providers/{provider_id}/oauth/refresh. The body
// is optional: naming an endpoint_id refreshes that account, and an absent or
// empty body refreshes every account whose token is due.
func (h *OAuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req schema.OAuthRefreshRequest
	if r.ContentLength != 0 {
		if err := schema.DecodeJSON(r, &req); err != nil {
			schema.WriteError(w, err)
			return
		}
		if err := schema.ValidateStruct(req); err != nil {
			schema.WriteError(w, err)
			return
		}
	}
	outcome, err := h.flow.Refresh(r.Context(), r.PathValue("provider_id"), req.EndpointID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	resp := schema.OAuthRefreshResponse{
		Refreshed:   outcome.Refreshed,
		EndpointIDs: outcome.EndpointIDs,
	}
	if outcome.ExpiresAt != nil {
		resp.ExpiresAt = ptr(schema.Timestamp(*outcome.ExpiresAt))
	}
	if resp.EndpointIDs == nil {
		resp.EndpointIDs = []string{}
	}
	schema.WriteJSON(w, http.StatusOK, resp)
}

// toOAuthStatusResponse renders the per-endpoint token state, keeping nil
// expiries nil so the panel can tell "no expiry known" from a zero instant.
func toOAuthStatusResponse(status service.OAuthStatus) schema.OAuthStatusResponse {
	resp := schema.OAuthStatusResponse{
		ProviderID: status.ProviderID,
		Flow:       status.Flow,
		Endpoints:  make([]schema.OAuthEndpointStatus, 0, len(status.Endpoints)),
	}
	for _, endpoint := range status.Endpoints {
		row := schema.OAuthEndpointStatus{
			EndpointID:   endpoint.EndpointID,
			Label:        endpoint.Label,
			Status:       endpoint.Status,
			RefreshState: endpoint.RefreshState,
		}
		if endpoint.ExpiresAt != nil {
			row.ExpiresAt = ptr(schema.Timestamp(*endpoint.ExpiresAt))
		}
		if endpoint.LastRefreshAt != nil {
			row.LastRefreshAt = ptr(schema.Timestamp(*endpoint.LastRefreshAt))
		}
		resp.Endpoints = append(resp.Endpoints, row)
	}
	return resp
}
