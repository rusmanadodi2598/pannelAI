// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_jobtoken.go
// @for       Turning a Qoder Personal Access Token into the short-lived job token
//
//	the vendor's signed endpoints accept, and keeping it until it is near
//	its own expiry.
//
// @uses      bytes, context, crypto/sha256, encoding/hex, encoding/json, fmt, io,
//
//	net/http, strings, sync, time.
//
// @reason    A Personal Access Token cannot sign COSY: the vendor answers the
//
//	signed endpoints with a refusal, so every Qoder call that carries a
//	`pt-` credential must exchange it first. The exchange is cheap to
//	do once and expensive to do per request, so the lifetime the vendor
//	states is what decides the reuse, with a buffer so a call never starts
//	on a token that expires mid-flight.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The exchange's own budget. The token it returns lives for a day, so five
// minutes of headroom is what keeps a request from starting on a token that would
// expire before the vendor finished answering it.
const (
	qoderJobTokenBuffer     = 5 * time.Minute
	qoderJobTokenDefaultTTL = 24 * time.Hour
	qoderExchangeTimeout    = 15 * time.Second
	qoderExchangeBodyLimit  = 1 << 20
)

// QoderJobToken exchanges a Personal Access Token for a job token, and caches the
// result until near expiry. A connector that already holds a device token does not
// consult it at all, which is why it is a separate seam rather than part of signing.
type QoderJobToken interface {
	// JobToken returns a token the signed endpoints accept for this Personal
	// Access Token. An error means no token could be obtained; a caller must not
	// fall back to presenting the Personal Access Token itself, because the
	// endpoints refuse it and the failure would move somewhere less obvious.
	JobToken(ctx context.Context, personalToken string) (string, error)
}

// qoderJobTokenClient is the HTTP implementation, safe for concurrent use: the
// cache is guarded, and the http.Client it carries is itself concurrency-safe.
type qoderJobTokenClient struct {
	client   *http.Client
	exchange string
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]cachedJobToken
}

// cachedJobToken is one exchanged token and the instant it stops being usable,
// which already carries the refresh buffer.
type cachedJobToken struct {
	token      string
	validUntil time.Time
}

// NewQoderJobTokenClient wires the exchanger to a provider's openapi base URL. The
// base comes from the registry entry, so intl and CN each reach their own exchange
// endpoint and nothing here infers a region from a token string. The client is the
// caller's, which is how the composition root keeps this call behind the same egress
// guard and proxy route as the rest of the provider's traffic.
func NewQoderJobTokenClient(baseURL string, client *http.Client) (*qoderJobTokenClient, error) {
	base := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("provider: qoder job-token exchange needs an openapi base url")
	}
	if client == nil {
		client = &http.Client{Timeout: qoderExchangeTimeout}
	}
	return &qoderJobTokenClient{
		client:   client,
		exchange: base + qoderJobTokenExchangePath,
		now:      time.Now,
		cache:    map[string]cachedJobToken{},
	}, nil
}

// qoderJobTokenExchangePath is the vendor's exchange route, spelled here because the
// openapi base URL is the configurable half of it.
const qoderJobTokenExchangePath = "/api/v1/jobToken/exchange"

// exchangeBody is the request the vendor reads, and the answer it writes, in one
// shape: the same field names go both ways.
type exchangeBody struct {
	PersonalToken string `json:"personal_token,omitempty"`
	Token         string `json:"token,omitempty"`
	RefreshToken  string `json:"refresh_token,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	ExpiresIn     int64  `json:"expires_in,omitempty"`
	Message       string `json:"message,omitempty"`
}

// JobToken returns a usable job token for this Personal Access Token, exchanging it
// on the first call and reusing it while the vendor's stated lifetime lasts.
func (c *qoderJobTokenClient) JobToken(ctx context.Context, personalToken string) (string, error) {
	pat := strings.TrimSpace(personalToken)
	if pat == "" {
		return "", fmt.Errorf("provider: qoder needs a personal access token to exchange")
	}
	key := hashCredentialToken(pat)
	if cached, ok := c.read(key, c.now()); ok {
		return cached, nil
	}

	answer, err := c.request(ctx, pat)
	if err != nil {
		return "", err
	}
	until := c.validUntil(answer)
	c.write(key, answer.Token, until)
	return answer.Token, nil
}

// request performs one exchange. The vendor's own client identifies itself as the
// CLI and states the IDE version and client type on the plain JSON call — this is
// the one Qoder request that is not COSY-signed, because a Personal Access Token
// cannot produce a signature.
func (c *qoderJobTokenClient) request(ctx context.Context, pat string) (exchangeBody, error) {
	raw, err := json.Marshal(exchangeBody{PersonalToken: pat})
	if err != nil {
		return exchangeBody{}, fmt.Errorf("provider: the qoder exchange request could not be encoded")
	}
	callCtx, cancel := context.WithTimeout(ctx, qoderExchangeTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.exchange, bytes.NewReader(raw))
	if err != nil {
		return exchangeBody{}, fmt.Errorf("provider: the qoder exchange request could not be built")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "qodercli/"+qoderIDEVersion)
	request.Header.Set("Cosy-Version", qoderIDEVersion)
	request.Header.Set("Cosy-ClientType", qoderClientType)

	response, err := c.client.Do(request)
	if err != nil {
		return exchangeBody{}, fmt.Errorf("provider: the qoder exchange call failed: %w", err)
	}
	defer func() {
		// reason: the body is read in full below, so a close error adds nothing.
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, qoderExchangeBodyLimit))
	if err != nil {
		return exchangeBody{}, fmt.Errorf("provider: the qoder exchange answer could not be read: %w", err)
	}
	var answer exchangeBody
	if err := json.Unmarshal(body, &answer); err != nil {
		return exchangeBody{}, fmt.Errorf("provider: the qoder exchange answer could not be decoded")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || strings.TrimSpace(answer.Token) == "" {
		reason := strings.TrimSpace(answer.Message)
		if reason == "" {
			reason = fmt.Sprintf("http %d", response.StatusCode)
		}
		return exchangeBody{}, fmt.Errorf("provider: the qoder personal access token was not accepted: %s", reason)
	}
	return answer, nil
}

// validUntil turns the vendor's expiry into a reuse bound. `expires_at` is what the
// service actually sends and is authoritative; `expires_in` is read as milliseconds,
// because measured against a day-long lifetime that is what the field means.
func (c *qoderJobTokenClient) validUntil(answer exchangeBody) time.Time {
	now := c.now()
	if answer.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339, answer.ExpiresAt); err == nil {
			return parsed.Add(-qoderJobTokenBuffer)
		}
	}
	if answer.ExpiresIn > 0 {
		return now.Add(time.Duration(answer.ExpiresIn) * time.Millisecond).Add(-qoderJobTokenBuffer)
	}
	return now.Add(qoderJobTokenDefaultTTL - qoderJobTokenBuffer)
}

// read returns a cached token only while it is still inside its bound.
func (c *qoderJobTokenClient) read(key string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.cache[key]
	if !ok || !now.Before(cached.validUntil) {
		return "", false
	}
	return cached.token, true
}

func (c *qoderJobTokenClient) write(key, token string, validUntil time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = cachedJobToken{token: token, validUntil: validUntil}
}

// hashCredentialToken keys the cache by a digest of the token rather than by the token
// itself, so a heap snapshot of the gateway does not carry every stored Personal
// Access Token in plain text. The cache is still per-token: the digest is stable for
// one credential and different for another.
func hashCredentialToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
