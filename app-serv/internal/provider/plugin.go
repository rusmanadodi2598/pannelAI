// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin.go
// @for       The Provider interface, its request context, and the outcome a
//
//	connector reports back to the core.
//
// @uses      internal/registry for the provider entry a connector is bound to.
// @reason    Every provider differs in how it connects: some take an API key,
//
//	some an OAuth flow, some need no credential, and some accept both.
//	The core must not learn those differences, or a fix for one
//	provider becomes a change to shared code. A connector per provider
//	keeps the difference inside one package, which is what makes a
//	provider patchable in isolation and a new provider addable without
//	touching the core.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package provider

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Family is which kind of credential an account holds. It is decided once, from
// the account's own auth type, because a provider may read a different header
// per family: choosing the header and choosing the value separately is how an
// OAuth token ends up in a static-key header.
//
// It is exported because the caller assembling a credential lives in the data
// plane, not in this package.
type Family int

const (
	// FamilyUnset means no credential material was supplied.
	FamilyUnset Family = iota

	// FamilyStaticKey is a long-lived key an operator pasted in.
	FamilyStaticKey

	// FamilyOAuth is a token obtained by an authorization flow.
	FamilyOAuth
)

// String names the family, so an error message that reports which header was
// missing says "oauth" or "static key" rather than an integer a reader has to
// decode against the constant list.
func (f Family) String() string {
	switch f {
	case FamilyStaticKey:
		return "static key"
	case FamilyOAuth:
		return "oauth"
	default:
		return "no credential"
	}
}

// Credential is what one upstream account presents to its provider. It is the
// single shape every auth family reduces to, so the core never branches on
// which family is in use.
//
// Exactly ONE of APIKey and AccessToken may be set. Setting both is the bug this
// type exists to prevent: a provider that routes OAuth through one header and
// static keys through another would then be given two contradictory signals, and
// whichever check ran first would decide the placement while the other decided
// the value. `Family` states which one the caller means, so there is nothing to
// infer.
//
// The fields hold plaintext for the duration of one request only. They are
// assembled from encrypted storage by the caller and never persisted here.
type Credential struct {
	// EndpointID and KeyID identify the account and, for a multi-key endpoint,
	// the exact key that was picked. They are what accounting records and what
	// a failure is attributed to.
	EndpointID string
	KeyID      string

	// APIKey is a static secret. It is set only for FamilyStaticKey.
	APIKey string

	// AccessToken is a token obtained by an authorization flow. It is set only
	// for FamilyOAuth.
	AccessToken string
	// Family states which credential kind this account presents. FamilyUnset
	// means no credential material is present, which is the correct state for a
	// provider that needs none.
	Family Family

	// Account and ProjectID are the non-secret identity fields some providers
	// need on the wire (a workspace id, a cloud project).
	Account   string
	ProjectID string

	// Metadata carries the provider-specific, non-secret values a connector
	// needs: a region, a client version, an editor identity. It is a map rather
	// than a typed struct because only the owning connector interprets it, and
	// a shared struct would accumulate every provider's fields.
	Metadata map[string]string
}

// StaticKey builds a credential presenting a long-lived key.
func StaticKey(endpointID, keyID, value string) Credential {
	return Credential{EndpointID: endpointID, KeyID: keyID, APIKey: value, Family: FamilyStaticKey}
}

// OAuthToken builds a credential presenting a token from an authorization flow.
func OAuthToken(endpointID, keyID, value string) Credential {
	return Credential{EndpointID: endpointID, KeyID: keyID, AccessToken: value, Family: FamilyOAuth}
}

// NoCredential builds a credential for a provider that needs none.
func NoCredential(endpointID string) Credential {
	return Credential{EndpointID: endpointID}
}

// family reports which credential kind is in use and its value. An explicitly
// declared Family wins; otherwise the single populated field decides, and a
// caller that set both fields without declaring a family is resolved to the
// OAuth token, which is the shorter-lived credential and the one an account
// configured for a flow holds.
func (c Credential) family() (Family, string) {
	switch c.Family {
	case FamilyStaticKey:
		return FamilyStaticKey, c.APIKey
	case FamilyOAuth:
		return FamilyOAuth, c.AccessToken
	}
	switch {
	case c.AccessToken != "":
		return FamilyOAuth, c.AccessToken
	case c.APIKey != "":
		return FamilyStaticKey, c.APIKey
	default:
		return FamilyUnset, ""
	}
}

// HasCredential reports whether any credential material is present. A
// credential-free provider returns false and the core must not treat that as an
// error.
func (c Credential) HasCredential() bool {
	_, value := c.family()
	return value != ""
}

// Request is one call the core asks a connector to perform. It carries the
// already-translated body and the resolved upstream target, so a connector only
// decides how to authenticate and which URL to use.
type Request struct {
	// Provider is the registry entry the call belongs to.
	Provider registry.Provider

	// Model is the resolved model, exposed so a connector can apply a
	// per-model rule (an upstream id override, a region from the model id).
	Model registry.Model

	// Body is the upstream-shaped request payload. Translation happened before
	// this point, so a connector must not reinterpret it.
	Body []byte

	// Stream reports whether the caller asked for a streamed response. A
	// provider that only streams is told so here rather than guessing.
	Stream bool

	// Headers are the request headers already assembled from the registry entry.
	// A connector may add to them, never replace them.
	Headers http.Header
}

// Response is what a connector returns. The body is streamed rather than
// buffered, because a chat completion can be long and the core forwards it
// without needing the whole payload.
type Response struct {
	// Status is the upstream HTTP status.
	Status int

	// Header carries the upstream response headers the caller may rely on
	// (retry hints, content type).
	Header http.Header

	// Body must be closed by the caller.
	Body ReadCloser

	// Usage is the accounting a connector can read without parsing the body:
	// some providers report quota on a header, and some report tokens in a
	// trailer. A zero value means "unknown", not "zero tokens".
	Usage Usage
}

// Usage is the token accounting one upstream call reported.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int

	// Reported distinguishes "the upstream said zero" from "the upstream said
	// nothing". Accounting must not record a fabricated zero as a measurement.
	Reported bool
}

// ReadCloser is the minimal read surface a response body needs. It is declared
// here rather than using io.ReadCloser directly so the interface stays explicit
// about what a connector must return.
type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

// Plugin is what a provider package implements. One connector binds to exactly
// one provider id and owns every choice that is specific to it: how to
// authenticate, which URL to call, and how to interpret what comes back.
//
// A connector is constructed once at boot and shared by concurrent requests, so
// an implementation MUST be safe for concurrent use and MUST NOT keep
// per-request state on itself.
type Plugin interface {
	// ProviderID is the registry id this connector handles. It is how the
	// registry finds the connector, so it must match the embedded entry exactly.
	ProviderID() string

	// AuthType reports how this provider authenticates. A provider that accepts
	// both a key and a flow reports the one its endpoint uses by default; the
	// endpoint's own auth type always wins at call time.
	AuthType() string

	// Endpoint returns the absolute URL to call for this request. A provider
	// with several regional hosts or a suffix rule decides that here, which is
	// why the core never builds a URL itself.
	Endpoint(req Request, cred Credential) (string, error)

	// ApplyAuth places the credential on the request. It is the only place that
	// knows which header this provider reads and in what scheme, which is the
	// whole reason the core does not hardcode a bearer token.
	ApplyAuth(req *http.Request, cred Credential) error

	// DecodeUsage extracts accounting from a response the connector already
	// understands. Returning a zero Usage means the connector could not tell,
	// which is not an error.
	DecodeUsage(status int, header http.Header) Usage

	// ShouldRetry reports whether an upstream outcome is worth another attempt
	// against the same target, and how long to wait. The core handles moving to
	// the next endpoint; this decides whether the same one deserves a retry.
	ShouldRetry(status int, header http.Header) RetryDecision

	// IsQuotaError reports whether an upstream rejection means this account has
	// exhausted its quota, as opposed to a transient failure. A quota rejection
	// parks the account; a transient one only backs off the key.
	IsQuotaError(status int, body []byte) bool
}

// RetryDecision is a connector's answer about retrying one target.
type RetryDecision struct {
	Retry bool

	// After is the wait before the next attempt. A zero value means retry
	// immediately, which is only appropriate when the connector knows the
	// failure was instantaneous.
	After time.Duration
}

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
