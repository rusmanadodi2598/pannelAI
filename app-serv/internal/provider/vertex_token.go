// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/vertex_token.go
// @for       Exchanging a service-account assertion for the access token a Google call presents, and keeping it until it is near its own expiry.
// @uses      bytes, context, encoding/json, fmt, io, net/http, net/url, strings, sync, time, internal/provider (the assertion).
// @reason    The assertion is cheap to sign once and wasteful to sign per request, and Google's own token lives for an hour, so the lifetime it states is what decides the reuse, with a buffer so a call never starts on a token that expires mid-flight. The exchange is the one outbound call a service-account credential makes, so it takes the caller's egress-guarded client rather than building one (docs/RULLES/SSRF.md §2.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The exchange's own budget. Google answers `expires_in` seconds; the buffer is
// what keeps a request from starting on a token that would expire before the
// upstream finished answering it.
const (
	vertexTokenBuffer     = 5 * time.Minute
	vertexTokenDefaultTTL = time.Hour
	vertexTokenTimeout    = 15 * time.Second
	vertexTokenBodyLimit  = 1 << 20
)

// VertexToken mints the access token a service-account credential presents to
// Google. A caller that cannot obtain one must not fall back to presenting the
// JSON itself: the endpoints refuse it, and the failure would move to a header
// dump rather than staying a credential error.
type VertexToken interface {
	// AccessToken returns a token the Google APIs accept for this
	// service-account JSON. An empty or malformed key is an error, not a
	// request that proceeds anonymously.
	AccessToken(ctx context.Context, serviceAccountJSON string) (string, error)
}

// vertexTokenClient is the HTTP implementation, safe for concurrent use: the
// cache is guarded, and the http.Client it carries is itself concurrency-safe.
type vertexTokenClient struct {
	client   *http.Client
	tokenURL string
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]cachedVertexToken
}

// cachedVertexToken is one minted token and the instant it stops being usable,
// which already carries the refresh buffer.
type cachedVertexToken struct {
	token      string
	validUntil time.Time
}

// NewVertexTokenClient wires the exchange to the process's egress-guarded HTTP
// client. A nil client is refused rather than defaulted, for the reason the
// Qoder exchanger refuses one: a client built here would carry no guard and no
// redirect rule, so a customer's private key could be re-sent across a 30x to a
// host the registry never named (docs/RULLES/SSRF.md §2.1).
func NewVertexTokenClient(client *http.Client) (*vertexTokenClient, error) {
	if client == nil {
		return nil, fmt.Errorf("provider: the service-account exchange needs the process egress-guarded client")
	}
	return &vertexTokenClient{
		client:   client,
		tokenURL: vertexTokenURL,
		now:      time.Now,
		cache:    map[string]cachedVertexToken{},
	}, nil
}

// vertexTokenAnswer is the token endpoint's answer. The error members are read
// too: a refused grant answers 400 with a reason in the body, and that reason
// is what tells an operator the key is wrong rather than the network.
type vertexTokenAnswer struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// AccessToken returns a usable token for this service-account JSON, exchanging
// the signed assertion on the first call and reusing the result while Google's
// stated lifetime lasts.
func (c *vertexTokenClient) AccessToken(ctx context.Context, serviceAccountJSON string) (string, error) {
	account, key, err := parseVertexServiceAccount(serviceAccountJSON)
	if err != nil {
		return "", err
	}
	cacheKey := hashCredentialToken(strings.TrimSpace(serviceAccountJSON))
	if cached, ok := c.read(cacheKey, c.now()); ok {
		return cached, nil
	}

	assertion, err := vertexAssertion(account, key, c.now())
	if err != nil {
		return "", err
	}
	answer, err := c.exchange(ctx, assertion)
	if err != nil {
		return "", err
	}
	c.write(cacheKey, answer.AccessToken, c.validUntil(answer))
	return answer.AccessToken, nil
}

// exchange performs the one outbound call: the assertion is posted as the
// jwt-bearer grant, which is the form Google's own client libraries use.
func (c *vertexTokenClient) exchange(ctx context.Context, assertion string) (vertexTokenAnswer, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	callCtx, cancel := context.WithTimeout(ctx, vertexTokenTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return vertexTokenAnswer{}, fmt.Errorf("provider: the service-account exchange request could not be built")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return vertexTokenAnswer{}, fmt.Errorf("provider: the service-account exchange call failed: %w", err)
	}
	defer func() {
		// reason: the body is read in full below, so a close error adds nothing.
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, vertexTokenBodyLimit))
	if err != nil {
		return vertexTokenAnswer{}, fmt.Errorf("provider: the service-account exchange answer could not be read: %w", err)
	}
	var answer vertexTokenAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return vertexTokenAnswer{}, fmt.Errorf("provider: the service-account exchange answer could not be decoded")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || strings.TrimSpace(answer.AccessToken) == "" {
		return vertexTokenAnswer{}, fmt.Errorf("provider: the service-account key was not accepted: %s", vertexRefusalReason(response.StatusCode, answer))
	}
	return answer, nil
}

// vertexRefusalReason picks the most specific fact the answer carries, so the
// error names Google's reason rather than an opaque status. It never echoes the
// assertion or the key.
func vertexRefusalReason(status int, answer vertexTokenAnswer) string {
	if reason := strings.TrimSpace(answer.ErrorDescription); reason != "" {
		return reason
	}
	if reason := strings.TrimSpace(answer.Error); reason != "" {
		return reason
	}
	return fmt.Sprintf("http %d", status)
}

// validUntil turns Google's stated lifetime into a reuse bound, with the
// default standing in when the answer states none.
func (c *vertexTokenClient) validUntil(answer vertexTokenAnswer) time.Time {
	now := c.now()
	if answer.ExpiresIn > 0 {
		return now.Add(time.Duration(answer.ExpiresIn) * time.Second).Add(-vertexTokenBuffer)
	}
	return now.Add(vertexTokenDefaultTTL - vertexTokenBuffer)
}

// read returns a cached token only while it is still inside its bound.
func (c *vertexTokenClient) read(key string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.cache[key]
	if !ok || !now.Before(cached.validUntil) {
		return "", false
	}
	return cached.token, true
}

func (c *vertexTokenClient) write(key, token string, validUntil time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = cachedVertexToken{token: token, validUntil: validUntil}
}
