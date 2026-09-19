// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/oauth_callback.go
// @for       The public OAuth callback: the browser redirect and the headless
//
//	JSON answer (SPEC-API-001 §7.4).
//
// @uses      internal/domain, internal/schema, internal/service, net/http,
//
//	net/url, strings.
//
// @reason    §7.4 makes this the one route a provider's browser redirect can
//
//	reach, so it cannot require a session, and it must answer two
//	audiences with one handler: a browser that needs to land back in
//	the panel, and a headless caller that asked for JSON with Accept.
//	The browser branch also has to choose a redirect origin without
//	trusting the request, which is why the configured base URL wins over
//	anything the flow staged.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// The query the panel reads on the page the callback lands on: `oauth` is the
// outcome, `oauth_error` carries the English reason when it is an error, and
// `endpoint_id` names the account the flow wrote.
const (
	oauthQueryOutcome  = "oauth"
	oauthQueryReason   = "oauth_error"
	oauthQueryEndpoint = "endpoint_id"
	oauthOutcomeOK     = "connected"
	oauthOutcomeFail   = "error"
)

// Callback serves GET /api/v1/providers/{provider_id}/oauth/callback. It
// consumes the staged state exactly once, then answers either the browser that
// followed the provider's redirect or a headless caller that asked for JSON.
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	providerID := r.PathValue("provider_id")
	query := r.URL.Query()
	connect, err := h.flow.Callback(r.Context(), service.OAuthCallbackInput{
		ProviderID:       providerID,
		Code:             query.Get("code"),
		State:            query.Get("state"),
		Error:            query.Get("error"),
		ErrorDescription: query.Get("error_description"),
	})

	origin := h.redirectOrigin(connect.RedirectBase)
	if wantsJSON(r) || origin == "" {
		// Either the caller asked for JSON, or there is no safe origin to send
		// a browser to (no configured base URL and nothing the flow staged).
		// Answering JSON in the second case is honest: guessing a host from the
		// request would be the open redirect this route refuses.
		if err != nil {
			schema.WriteError(w, err)
			return
		}
		schema.WriteJSON(w, http.StatusOK, connectResult(connect))
		return
	}
	http.Redirect(w, r, panelOutcomeURL(origin, providerID, connect, err), http.StatusFound)
}

// connectResult renders the headless answer: the account that now stands for
// the connection and a token hint, never the tokens themselves.
func connectResult(connect service.OAuthConnect) schema.OAuthConnectResult {
	return schema.OAuthConnectResult{
		EndpointID:   connect.Endpoint.ID(),
		ProviderID:   connect.Endpoint.ProviderID(),
		Label:        connect.Endpoint.Label(),
		AccountEmail: connect.Endpoint.Account().Email,
		TokenHint:    connect.TokenHint,
		Created:      connect.Created,
	}
}

// redirectOrigin picks the origin the browser is sent back to. A configured
// base URL is authoritative; the origin the flow staged is only a fallback, and
// it is dropped unless it is an absolute http(s) URL, so neither a request
// header nor a caller-supplied redirect can turn this route into an open
// redirect (OWASP A01).
func (h *OAuthHandler) redirectOrigin(staged string) string {
	if h.baseURL != "" {
		return h.baseURL
	}
	parsed, err := url.Parse(strings.TrimSpace(staged))
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// panelOutcomeURL builds the provider detail URL the panel reads: the outcome
// code always, the endpoint the flow wrote on success, and the English reason
// on failure. The reason is the AppError's own message, never the wrapped
// chain, so nothing internal rides a URL (AGENTS.md §1.3).
func panelOutcomeURL(origin, providerID string, connect service.OAuthConnect, err error) string {
	target := origin + "/providers/" + url.PathEscape(providerID)
	query := url.Values{}
	if err != nil {
		query.Set(oauthQueryOutcome, oauthOutcomeFail)
		query.Set(oauthQueryReason, domain.AsAppError(err).Message)
		return target + "?" + query.Encode()
	}
	query.Set(oauthQueryOutcome, oauthOutcomeOK)
	query.Set(oauthQueryEndpoint, connect.Endpoint.ID())
	return target + "?" + query.Encode()
}

// wantsJSON reports whether the caller asked for the JSON answer rather than a
// browser redirect (SPEC-API-001 §7.4).
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}
