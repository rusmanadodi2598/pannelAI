// Package oauthhttp performs the OAuth rounds the flow service orchestrates.
//
// @file      internal/service/oauthhttp/oauth_client_state_wire.go
// @for       The headers and URLs the state round's vendor reads its calls by.
//
// @uses      net/url, strings, time, internal/registry.
// @reason    CodeBuddy authenticates by shape rather than by secret: which host it
//
//	thinks it is talking to (`X-Domain`), that the caller is a plugin rather
//	than a browser (`X-Requested-With`, the `X-No-*` negations), and that
//	the refresh token rides a header. Each is a detail a generic OAuth client
//	would guess wrong, so they are stated once here and named. Kept apart
//	from the calls themselves so both files stay inside the AGENTS.md §1.1
//	budget and the wire contract can be read on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package oauthhttp

import (
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// The header values the vendor expects on every call of this family.
const (
	stateRequestedWith = "XMLHttpRequest"
	stateProductValue  = "SaaS"
	stateNegationValue = "true"
)

// errStatePending is the poll's "keep waiting" answer, carried as a sentinel so
// the one caller that can wait translates it into the flow's pending verdict
// rather than a failure the panel would report.
var errStatePending = &statePendingError{}

// statePendingError is a distinct type rather than a shared error value, so an
// equality test cannot match a different vendor's "pending".
type statePendingError struct{}

func (e *statePendingError) Error() string { return "authorization pending" }

// stateBaseHeaders are the headers every call in this family carries. `X-Domain`
// is not a third fact to store: the reference sets it to the provider's own host,
// which the entry already declares as its base URL.
func stateBaseHeaders(oauth *registry.OAuth) map[string]string {
	return map[string]string{
		"Accept":             "application/json",
		"User-Agent":         oauth.UserAgent,
		"X-Requested-With":   stateRequestedWith,
		"X-Domain":           stateDomain(oauth.BaseURL),
		"X-Product":          stateProductValue,
		"X-No-Authorization": stateNegationValue,
		"X-No-User-Id":       stateNegationValue,
	}
}

// stateRoundHeaders adds the body type the state POST is read with.
func stateRoundHeaders(oauth *registry.OAuth) map[string]string {
	headers := stateBaseHeaders(oauth)
	headers["Content-Type"] = "application/json"
	return headers
}

// statePollHeaders adds the two negations the token endpoint asks for on top of
// the ones every call carries: the poll identifies nobody but the round.
func statePollHeaders(oauth *registry.OAuth) map[string]string {
	headers := stateBaseHeaders(oauth)
	headers["X-No-Enterprise-Id"] = stateNegationValue
	headers["X-No-Department-Info"] = stateNegationValue
	return headers
}

// stateDomain reduces a base URL to the host the vendor echoes back in X-Domain.
func stateDomain(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}

// stateQuery appends one query parameter to a URL the entry declared, preserving
// any parameters it already carries. The round's two identifying values both
// travel this way, the platform at the state call, the state at the poll.
func stateQuery(endpoint, key, value string) string {
	target := strings.TrimSpace(endpoint)
	if target == "" || value == "" {
		return target
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return target
	}
	params := parsed.Query()
	params.Set(key, value)
	parsed.RawQuery = params.Encode()
	return parsed.String()
}

// statePollInterval converts the entry's millisecond cadence, defaulting to the
// reference's five seconds when it declares none.
func statePollInterval(oauth *registry.OAuth) time.Duration {
	if oauth.PollIntervalMS <= 0 {
		return 5 * time.Second
	}
	return time.Duration(oauth.PollIntervalMS) * time.Millisecond
}
