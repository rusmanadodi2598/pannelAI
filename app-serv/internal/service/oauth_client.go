// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_client.go
// @for       The OAuth token endpoint client: one grant call and one userinfo
//
//	call, each under an explicit deadline.
//
// @uses      internal/dataplane (shared HTTP pool), context, encoding/json,
//
//	net/http, strings, time.
//
// @reason    SPEC-API-001 §7.4 needs a code exchange and a refresh grant, and
//
//	§8.1 records that providers disagree on the grant body's encoding
//	(claude answers JSON bodies, the rest answer form encoding), so the
//	wire types live in oauth_grant.go and the identity decode in
//	oauth_identity.go. What stays here is the transport: every token URL
//	it calls comes from the embedded registry, never from a request, so
//	the surface has no SSRF seam (OWASP A01).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// grantCallTimeout bounds one token-endpoint round trip. A token endpoint that
// has not answered in 15 seconds will not answer usefully later in this request.
const grantCallTimeout = 15 * time.Second

// OAuthTokenClient is the outbound token-service seam. It is an interface so
// the flow and the refresh worker are driven by a fake in tests, with no token
// ever leaving the process (SPEC-API-001 §6: tokens exist in memory and
// sealed, nowhere else).
type OAuthTokenClient interface {
	Grant(ctx context.Context, tokenURL, encoding string, grant TokenGrant) (TokenResponse, error)
	UserInfo(ctx context.Context, infoURL, accessToken string) (OAuthIdentity, error)
}

// OAuthHTTPClient is the net/http implementation of OAuthTokenClient.
type OAuthHTTPClient struct {
	client *http.Client
}

// NewOAuthHTTPClient binds the token client to the shared pool, so an OAuth
// call carries the same §1.7 limits as every other outbound call. The caller
// supplies the process's guarded client; the default is the plain pool.
func NewOAuthHTTPClient(client *http.Client) *OAuthHTTPClient {
	if client == nil {
		client = dataplane.NewHTTPClient(dataplane.HTTPClientDeps{})
	}
	return &OAuthHTTPClient{client: client}
}

// Grant performs one token request. `encoding` selects the body: "json" for the
// providers whose token endpoints answer JSON bodies, anything else form
// encoding, which is what an OAuth endpoint that never declared a preference
// is required to accept.
func (c *OAuthHTTPClient) Grant(ctx context.Context, tokenURL, encoding string, grant TokenGrant) (TokenResponse, error) {
	var body []byte
	var contentType string
	if strings.EqualFold(encoding, "json") {
		encoded, err := json.Marshal(grantJSONObject(grant))
		if err != nil {
			return TokenResponse{}, domain.NewInternalError("the token request could not be encoded")
		}
		body, contentType = encoded, "application/json"
	} else {
		body, contentType = []byte(grant.Form().Encode()), "application/x-www-form-urlencoded"
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return TokenResponse{}, domain.NewValidationError("the token endpoint URL is invalid")
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")

	response, err := doGrant(c.client, request)
	if err != nil {
		return TokenResponse{}, err
	}
	return response, nil
}

// UserInfo reads the account identity behind an access token. A userinfo
// endpoint is optional per provider, so a failure to answer is reported to the
// caller, which decides whether the flow can proceed without an identity.
func (c *OAuthHTTPClient) UserInfo(ctx context.Context, infoURL, accessToken string) (OAuthIdentity, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return OAuthIdentity{}, domain.NewValidationError("the user info URL is invalid")
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")

	identity := OAuthIdentity{}
	response, err := c.client.Do(request)
	if err != nil {
		return OAuthIdentity{}, err
	}
	defer func() {
		// reason: the body is drained by the decoder below; a close error adds
		// nothing a caller could act on and the connection is released anyway.
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return OAuthIdentity{}, domain.NewUpstreamError("the user info endpoint rejected the request")
	}
	if err := json.NewDecoder(response.Body).Decode(&identity); err != nil {
		return OAuthIdentity{}, domain.NewUpstreamError("the user info answer could not be decoded")
	}
	return identity, nil
}
