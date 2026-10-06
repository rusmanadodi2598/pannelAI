// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/oauth_client_device.go
// @for       The device-token poll client: one GET per poll attempt, the pending verdicts, and the upstream's expiry hint parsing.
// @uses      net/http, encoding/json, strings, time, internal/domain.
// @reason    A device flow's poll is not a token grant: it is a GET whose 202/404 answers mean "keep waiting" and whose 200 body carries token material plus an expiry hint in any of three shapes. Keeping that translation here puts the wire tolerance in one place and leaves the flow service owning only the connect decision. The net/http import is egress only, so a worker can call this the same way a route does (AGENTS.md §1.5).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package oauthhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// DeviceTokenResponse is one successful device poll: the token set plus the
// identity and absolute expiry the upstream reported. ExpiresAt always carries
// a value, the parse falls back to the reference's thirty-day default, so
// the flow service never stores a token with an unknown lifetime.
type DeviceTokenResponse struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	ExpiresAt    time.Time
}

// devicePollBody is the upstream's 200 answer. expires_at arrives as a number
// or a string, so it rides as raw JSON and parseDeviceExpiry reads it.
type devicePollBody struct {
	Token        string          `json:"token"`
	RefreshToken string          `json:"refresh_token"`
	UserID       string          `json:"user_id"`
	ExpiresAt    json.RawMessage `json:"expires_at"`
	ExpiresIn    *int            `json:"expires_in"`
	Message      string          `json:"message"`
}

// devicePollPending statuses mean the user has not finished the browser step
// yet: 202 is the documented wait answer and 404 is what the upstream reports
// before it has registered the first poll.
var devicePollPending = map[int]bool{http.StatusAccepted: true, http.StatusNotFound: true}

// The reference's own tolerances, kept as names so the parse order below reads
// as the rules they are: the vendor answers this endpoint to its CLI, whose
// user agent the CDN expects; a token whose expiry the upstream never stated
// lives for thirty days; and no stored token is allowed to open already dead.
const (
	devicePollUserAgent = "Go-http-client/2.0"
	devicePollBodyLimit = 1 << 20
	deviceExpiryFloor   = 24 * time.Hour
	deviceExpiryDefault = 30 * 24 * time.Hour
)

// DevicePoll attempts one poll against the provider's device token endpoint.
// pending is true when the user has not authorized yet, so the caller keeps
// polling; a false pending with an error is a terminal failure.
func (c *OAuthHTTPClient) DevicePoll(ctx context.Context, deviceTokenURL, nonce, verifier string) (DeviceTokenResponse, bool, error) {
	if strings.TrimSpace(nonce) == "" || strings.TrimSpace(verifier) == "" {
		return DeviceTokenResponse{}, false, domain.NewValidationError("the device poll needs its nonce and verifier")
	}
	ctx, cancel := context.WithTimeout(ctx, grantCallTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, deviceTokenURL, nil)
	if err != nil {
		return DeviceTokenResponse{}, false, domain.NewValidationError("the device token URL is invalid")
	}
	query := request.URL.Query()
	query.Set("nonce", nonce)
	query.Set("verifier", verifier)
	query.Set("challenge_method", "S256")
	request.URL.RawQuery = query.Encode()
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", devicePollUserAgent)

	response, err := c.client.Do(request)
	if err != nil {
		return DeviceTokenResponse{}, false, err
	}
	defer func() {
		// reason: the body is fully read below (pending or parsed), so a close
		// error adds nothing the caller could act on.
		_ = response.Body.Close()
	}()

	if devicePollPending[response.StatusCode] {
		return DeviceTokenResponse{}, true, nil
	}

	raw, err := io.ReadAll(io.LimitReader(response.Body, devicePollBodyLimit))
	if err != nil {
		return DeviceTokenResponse{}, false, fmt.Errorf("reading the device poll answer: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := fmt.Sprintf("the device token endpoint refused the poll (HTTP %d)", response.StatusCode)
		var body devicePollBody
		if json.Unmarshal(raw, &body) == nil && body.Message != "" {
			message = fmt.Sprintf("the device token endpoint refused the poll: %s", body.Message)
		}
		return DeviceTokenResponse{}, false, domain.NewUpstreamError(message)
	}

	var body devicePollBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return DeviceTokenResponse{}, false, domain.NewUpstreamError("the device poll answer could not be decoded")
	}
	if strings.TrimSpace(body.Token) == "" {
		// A 200 without token material means the upstream changed shape; storing
		// nothing is safer than storing an empty credential.
		return DeviceTokenResponse{}, false, domain.NewUpstreamError("the device token endpoint answered without a token")
	}
	return DeviceTokenResponse{
		AccessToken:  body.Token,
		RefreshToken: body.RefreshToken,
		UserID:       body.UserID,
		ExpiresAt:    parseDeviceExpiry(body.ExpiresAt, body.ExpiresIn, time.Now()),
	}, false, nil
}

// parseDeviceExpiry converts the upstream's expiry hint into an absolute
// instant, in the reference's order: a numeric ms epoch (number or numeric
// string) first, so a short numeric string like "2026" is an epoch, not a
// year, then an RFC3339 string, then expires_in seconds counted from now,
// then the thirty-day default. expires_in of exactly zero means "already
// expired" and is honored here; the flow's floor decides what gets stored.
func parseDeviceExpiry(raw json.RawMessage, expiresIn *int, now time.Time) time.Time {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" && trimmed != "null" {
		if asNumber, err := strconv.ParseInt(trimmed, 10, 64); err == nil && asNumber > 0 {
			return time.UnixMilli(asNumber)
		}
		var asString string
		if json.Unmarshal(raw, &asString) == nil {
			if asNumber, err := strconv.ParseInt(strings.TrimSpace(asString), 10, 64); err == nil && asNumber > 0 {
				return time.UnixMilli(asNumber)
			}
			if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(asString)); err == nil {
				return parsed
			}
		}
	}
	if expiresIn != nil && *expiresIn >= 0 {
		return now.Add(time.Duration(*expiresIn) * time.Second)
	}
	return now.Add(deviceExpiryDefault)
}

// FloorDeviceExpiry applies the reference's one-day floor: an upstream that
// reports an expiry closer than a day out (or in the past) still stores a
// token with one day of life, so a skewed clock cannot void a fresh login.
func FloorDeviceExpiry(expires time.Time, now time.Time) time.Time {
	if floor := now.Add(deviceExpiryFloor); expires.Before(floor) {
		return floor
	}
	return expires
}
