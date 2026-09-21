// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin.go
// @for       The Plugin interface and the outcome a connector reports back to the
//
//	core.
//
// @uses      net/http, time.
// @reason    Every provider differs in how it connects: some take an API key,
//
//	some an OAuth flow, some need no credential, and some accept both.
//	The core must not learn those differences, or a fix for one
//	provider becomes a change to shared code. A connector per provider
//	keeps the difference inside one package, which is what makes a
//	provider patchable in isolation and a new provider addable without
//	touching the core. The credential and request shapes live in
//	plugin_credential.go and plugin_request.go, and the shared defaults
//	in plugin_base.go, for the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package provider

import (
	"net/http"
	"time"
)

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

// Transformer is the optional seam a connector implements when a provider needs
// the outbound request rewritten before it is sent (a required field, a decoy the
// upstream gates on, a field renamed for the wire). It is a separate interface
// rather than more methods on Plugin so a connector that needs none is unchanged:
// the core type-asserts for it, and a connector that does not implement it keeps
// the pass-through behaviour every provider had before.
//
// An implementation must be safe for concurrent use and must not keep
// per-request state, exactly as Plugin requires: it is built once at boot.
type Transformer interface {
	// TransformRequest rewrites the outbound request into the shape this
	// provider accepts. It receives the body after translation, so it may
	// adjust members but must not reinterpret the wire format.
	//
	// Returning an error refuses the call before anything is sent, which is the
	// right answer for a body the connector cannot read.
	TransformRequest(req *Request) error
}

// StreamForcer is the optional seam a connector implements when a provider
// refuses a non-streaming request. The core reads it to decide that a client
// which asked for one JSON body has to be served from a stream instead, which is
// a decision only the core can act on: it owns how the answer is read.
//
// Declaring it is enough for a provider whose only requirement is the stream
// flag, because the core forces that itself. A provider that needs more of its
// body rewritten also implements Transformer.
type StreamForcer interface {
	// ForcesStream reports whether this provider refuses a non-streaming
	// request.
	ForcesStream() bool
}

// RetryDecision is a connector's answer about retrying one target.
type RetryDecision struct {
	Retry bool

	// After is the wait before the next attempt. A zero value means retry
	// immediately, which is only appropriate when the connector knows the
	// failure was instantaneous.
	After time.Duration
}
