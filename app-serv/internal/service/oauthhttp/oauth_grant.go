// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/oauth_grant.go
// @for       The token-endpoint wire types: one grant rendered as form or as JSON, one typed answer, and the refusal mapping.
// @uses      context, encoding/json, net/http, net/url, strconv, strings, internal/domain.
// @reason    SPEC-API-001 §8.1 records that providers disagree on the grant body's encoding, so one grant type renders both ways and the disagreement stays at this boundary instead of reaching the flow.
//
//	Empty fields are omitted from either rendering, which is what lets
//	the same type carry a code exchange, a refresh grant, and a device
//	poll without a per-grant struct trio. The net/http import is egress only,
//
//	so a worker can call this the same way a route does (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package oauthhttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TokenGrant is one OAuth token request, whichever grant type it carries. Empty
// fields are omitted, so the same type serves the code exchange, the refresh
// grant, and a device poll without a per-grant struct trio.
type TokenGrant struct {
	GrantType    string
	Code         string
	RefreshToken string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	CodeVerifier string
	Scope        string
}

// TokenResponse is the typed answer of a token endpoint.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Form renders the grant as ordered form values, omitting empty fields so a
// refresh grant does not carry an authorization code it does not have.
func (g TokenGrant) Form() url.Values {
	values := url.Values{}
	for _, field := range g.fields() {
		if field.value != "" {
			values.Set(field.name, field.value)
		}
	}
	return values
}

// grantJSONObject renders the grant as a flat JSON object with the same
// omit-empty rule, keyed by the token-endpoint field names.
func grantJSONObject(grant TokenGrant) map[string]string {
	object := make(map[string]string, 8)
	for _, field := range grant.fields() {
		if field.value != "" {
			object[field.name] = field.value
		}
	}
	return object
}

type grantField struct {
	name, value string
}

func (g TokenGrant) fields() []grantField {
	return []grantField{
		{"grant_type", g.GrantType},
		{"code", g.Code},
		{"refresh_token", g.RefreshToken},
		{"client_id", g.ClientID},
		{"client_secret", g.ClientSecret},
		{"redirect_uri", g.RedirectURI},
		{"code_verifier", g.CodeVerifier},
		{"scope", g.Scope},
	}
}

// grantCall is the neutral shape of one token-endpoint request: method, URL,
// headers, and body. The seam speaks this rather than *http.Request, so the
// HTTP layer's own types stay out of the service boundary (AGENTS.md §1.5)
// and the transport details are built in one place.
type grantCall struct {
	method  string
	url     string
	headers map[string]string
	body    string
}

// doGrant performs the request and decodes the typed answer, mapping a refusal
// to an upstream error that carries the provider's own reason when it gave one.
func doGrant(ctx context.Context, client *http.Client, call grantCall) (TokenResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, grantCallTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, call.method, call.url, strings.NewReader(call.body))
	if err != nil {
		return TokenResponse{}, domain.NewValidationError("the token endpoint URL is invalid")
	}
	for name, value := range call.headers {
		request.Header.Set(name, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return TokenResponse{}, domain.NewUpstreamError("the token endpoint could not be reached")
	}
	defer func() {
		// reason: the body is drained by the decoder below; a close error adds
		// nothing a caller could act on and the connection is released anyway.
		_ = response.Body.Close()
	}()

	answer := TokenResponse{}
	if err := json.NewDecoder(io.LimitReader(response.Body, tokenBodyLimit)).Decode(&answer); err != nil {
		return TokenResponse{}, domain.NewUpstreamError(
			"the token endpoint answer could not be decoded (status " + strconv.Itoa(response.StatusCode) + ")")
	}
	if answer.Error != "" {
		message := answer.Error
		if answer.ErrorDescription != "" {
			message = message + ": " + answer.ErrorDescription
		}
		return TokenResponse{}, domain.NewUpstreamError("the token endpoint refused the grant: " + message)
	}
	if answer.AccessToken == "" {
		return TokenResponse{}, domain.NewUpstreamError("the token endpoint returned no access token")
	}
	return answer, nil
}
