// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_client_state.go
// @for       The state-round OAuth shape: the vendor-minted round, its poll, and
//
//	the refresh that rides a header instead of a form field.
//
// @uses      bytes, context, encoding/json, errors, fmt, io, net/http, strings,
//
//	time, internal/domain, internal/registry.
//
// @reason    CodeBuddy authorizes the way a device flow looks to an operator —
//
//	open a link, wait, get a token — but shares no wire with the PKCE round
//	the generic device client runs. The reference's own
//	`src/lib/oauth/providers/codebuddy-{cn,intl}.js` posts to the state
//	endpoint for a `state` plus the browser URL, polls the token endpoint by
//	`?state=`, and reads `code: 11217` as "not yet". None of that is
//	expressible through the existing grant or poll shape, so it lives here
//	rather than being bent into them, and the two regions differ only by
//	domain, user agent, and platform — which the registry entry already
//	carries.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// The state round's tolerances, all of them the reference's own values: the code
// that means "the operator has not finished the browser step yet", the lifetime a
// token gets when the vendor states none, the body ceiling, and the plugin header
// the refresh endpoint insists on.
const (
	statePendingCode      = 11217
	stateTokenDefaultTTL  = 24 * time.Hour
	stateBodyLimit        = 1 << 20
	stateRefreshSource    = "plugin"
	stateAnswerEnvelope   = "the state endpoint answered without a token"
	stateRoundRetryWindow = 300
)

// StateRound is one vendor-minted authorization round: the handle every later
// poll carries, and the URL the operator opens. There is no user code — the
// reference sends none — so the panel shows the link alone.
type StateRound struct {
	State    string
	AuthURL  string
	Interval time.Duration
	Expires  int
}

// StateRoundClient is the optional seam a provider whose authorization is minted
// by the vendor rather than locally asserts. It is a separate interface for the
// same reason provider.StreamForcer is one: the flow service is driven by a fake
// in every test, and a mandatory third method would force each of those fakes to
// answer a flow they never exercise.
type StateRoundClient interface {
	StateRound(ctx context.Context, oauth *registry.OAuth) (StateRound, error)
	StatePoll(ctx context.Context, oauth *registry.OAuth, state string) (DeviceTokenResponse, bool, error)
	StateRefresh(ctx context.Context, oauth *registry.OAuth, refreshToken string) (TokenResponse, error)
}

// stateEnvelope is the doubled Tencent shape every three calls share: a transport
// 200 that may still carry a refusal, so `code` and not the status is the verdict.
type stateEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type stateRoundBody struct {
	State   string `json:"state"`
	AuthURL string `json:"authUrl"`
}

type stateTokenBody struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

// StateRound asks the vendor to open a round. The platform the entry declares
// (CLI for the CN site, ide for the international one) is a query parameter
// rather than a body field, which is what the reference's URL string shows.
func (c *OAuthHTTPClient) StateRound(ctx context.Context, oauth *registry.OAuth) (StateRound, error) {
	target := stateQuery(oauth.StateURL, "platform", oauth.Platform)
	data, err := c.stateCall(ctx, http.MethodPost, target, stateRoundHeaders(oauth))
	if err != nil {
		return StateRound{}, err
	}
	var body stateRoundBody
	if err := json.Unmarshal(data, &body); err != nil {
		return StateRound{}, domain.NewUpstreamError("the state round could not be decoded")
	}
	if strings.TrimSpace(body.State) == "" || strings.TrimSpace(body.AuthURL) == "" {
		return StateRound{}, domain.NewUpstreamError("the state endpoint returned no state or authorization URL")
	}
	return StateRound{
		State: body.State, AuthURL: body.AuthURL,
		Interval: statePollInterval(oauth), Expires: stateRoundRetryWindow,
	}, nil
}

// StatePoll attempts one poll. A code 11217 answer is the vendor saying "not
// yet", which is a wait rather than a failure; anything else that is not a token
// ends the round.
func (c *OAuthHTTPClient) StatePoll(ctx context.Context, oauth *registry.OAuth, state string) (DeviceTokenResponse, bool, error) {
	if strings.TrimSpace(state) == "" {
		return DeviceTokenResponse{}, false, domain.NewValidationError("the state poll needs its state")
	}
	target := stateQuery(oauth.TokenURL, "state", state)
	data, err := c.stateCall(ctx, http.MethodGet, target, statePollHeaders(oauth))
	if err != nil {
		// The vendor's pending code is a wait, not a failure: the caller keeps the
		// round staged and polls again on its own cadence.
		if errors.Is(err, errStatePending) {
			return DeviceTokenResponse{}, true, nil
		}
		return DeviceTokenResponse{}, false, err
	}
	var body stateTokenBody
	if err := json.Unmarshal(data, &body); err != nil {
		return DeviceTokenResponse{}, false, domain.NewUpstreamError("the state poll answer could not be decoded")
	}
	if strings.TrimSpace(body.AccessToken) == "" {
		return DeviceTokenResponse{}, false, domain.NewUpstreamError(stateAnswerEnvelope)
	}
	life := stateTokenDefaultTTL
	if body.ExpiresIn > 0 {
		life = time.Duration(body.ExpiresIn) * time.Second
	}
	return DeviceTokenResponse{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		ExpiresAt:    time.Now().Add(life),
	}, false, nil
}

// StateRefresh renews a token the same way the reference does: the refresh token
// travels in a header and the body stays empty, so the standard form grant would
// not reach this endpoint at all.
func (c *OAuthHTTPClient) StateRefresh(ctx context.Context, oauth *registry.OAuth, refreshToken string) (TokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return TokenResponse{}, domain.NewValidationError("the state refresh needs a refresh token")
	}
	if strings.TrimSpace(oauth.RefreshURL) == "" {
		return TokenResponse{}, domain.NewValidationError("the provider declares no refresh endpoint")
	}
	headers := stateRoundHeaders(oauth)
	headers["X-Refresh-Token"] = refreshToken
	headers["X-Auth-Refresh-Source"] = stateRefreshSource
	data, err := c.stateCall(ctx, http.MethodPost, oauth.RefreshURL, headers)
	if err != nil {
		return TokenResponse{}, err
	}
	var body stateTokenBody
	if err := json.Unmarshal(data, &body); err != nil {
		return TokenResponse{}, domain.NewUpstreamError("the refresh answer could not be decoded")
	}
	if strings.TrimSpace(body.AccessToken) == "" {
		return TokenResponse{}, domain.NewUpstreamError(stateAnswerEnvelope)
	}
	return TokenResponse{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken, ExpiresIn: body.ExpiresIn}, nil
}

// stateCall performs one call and returns the envelope's payload, turning the
// vendor's own refusal into the error the operator reads. A pending code is not an
// error: it answers with a nil payload and a false verdict so the caller polls on.
func (c *OAuthHTTPClient) stateCall(ctx context.Context, method, target string, headers map[string]string) (json.RawMessage, error) {
	var body io.Reader
	if method == http.MethodPost {
		body = bytes.NewBufferString("{}")
	}
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, domain.NewValidationError("the authorization endpoint URL is invalid")
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			request.Header.Set(key, value)
		}
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		// reason: the body is read in full below, so a close error reports nothing
		// the caller can act on.
		_ = response.Body.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(response.Body, stateBodyLimit))
	if err != nil {
		return nil, fmt.Errorf("reading the authorization answer: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, domain.NewUpstreamError(fmt.Sprintf(
			"the authorization endpoint refused the request (HTTP %d)", response.StatusCode))
	}
	var envelope stateEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, domain.NewUpstreamError("the authorization answer could not be decoded")
	}
	if envelope.Code == statePendingCode {
		return nil, errStatePending
	}
	if envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Msg)
		if message == "" {
			message = "unknown"
		}
		return nil, domain.NewUpstreamError("the authorization endpoint refused the request: " + message)
	}
	return envelope.Data, nil
}
