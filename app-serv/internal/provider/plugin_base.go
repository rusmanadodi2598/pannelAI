// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_base.go
// @for       The connector behaviour shared by every provider.
// @uses      net/http, strconv, strings, time.
// @reason    A connector should only override what actually differs, so the
//
//	conservative answer to every optional question lives here. For a plain
//	OpenAI-compatible vendor that means only ProviderID is strictly
//	required, which is what keeps the registry from needing one file per
//	vendor. The optional seams a connector may add are declared here too,
//	so the core reads one place to learn what a connector can do.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Base is the connector behaviour shared by every provider, embedded by each
// implementation so a connector only overrides what actually differs. This is
// what keeps a new provider small: for a plain OpenAI-compatible vendor, only
// ProviderID is strictly required.
type Base struct {
	// ID is the provider id this connector serves.
	ID string

	// Auth is the default auth type for this provider.
	Auth string

	// Format is the wire format, used to report the provider's family.
	Format string
}

// ProviderID implements Plugin.
func (b Base) ProviderID() string { return b.ID }

// AuthType implements Plugin.
func (b Base) AuthType() string { return b.Auth }

// DecodeUsage implements Plugin with the conservative answer: no accounting.
// A connector that can read real numbers overrides it.
func (Base) DecodeUsage(int, http.Header) Usage { return Usage{} }

// ShouldRetry implements Plugin with the transport-level default: retry the
// rate-limited and server-error statuses, honouring a Retry-After header.
func (Base) ShouldRetry(status int, header http.Header) RetryDecision {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return RetryDecision{Retry: true, After: retryAfter(header)}
	default:
		return RetryDecision{}
	}
}

// IsQuotaError implements Plugin with the common signal: a payment-required or
// quota-exhausted status parks the account.
func (Base) IsQuotaError(status int, _ []byte) bool {
	return status == http.StatusPaymentRequired || status == http.StatusForbidden
}

// retryAfter reads a Retry-After header in either of its two permitted forms.
func retryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return 0
}
