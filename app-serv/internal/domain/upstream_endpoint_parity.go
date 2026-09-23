// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_parity.go
// @for       The five connection-parity fields on an endpoint, and the rules
//
//	each one carries.
//
// @uses      internal/domain (AppError constructors), strings, time.
// @reason    Draft 017 §4.1b lists five fields a reference "connection" has and an
//
//	endpoint did not: a cross-provider order, a default model, a
//	consecutive-use counter, the last error that was not a test result, and
//	a proxy binding. They are declared here rather than on the aggregate
//	because each one carries a rule — a validated range, a run length, a
//	credential scrub — and AGENTS.md §2.2 keeps rules on the type rather
//	than at the call site.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-23
package domain

import (
	"strings"
	"time"
)

// Parity-field bounds. Each is a stored column, so the limit is stated here
// rather than discovered as a database truncation.
const (
	maxDefaultModelLength = 200
	maxProxyPoolIDLength  = 64
	maxLastErrorMessage   = 500
)

// credentialScrubbedMessage replaces any upstream error text that looks like it
// carries a credential. It is a fixed English sentence so a client can render it
// and an operator knows what happened without seeing the secret.
const credentialScrubbedMessage = "the upstream rejected this credential"

// EndpointParity is the load-path carrier for the parity fields, so
// RehydrateUpstreamEndpoint does not take seven more positional arguments.
//
// It is a plain value with exported fields on purpose: it is a transport for
// stored data, not an aggregate, and the rules live on UpstreamEndpoint's
// methods. It is never returned to a caller outside this package.
type EndpointParity struct {
	GlobalPriority      int
	DefaultModel        string
	ConsecutiveUseCount int
	LastErrorCode       string
	LastErrorMessage    string
	LastErrorAt         *time.Time
	ProxyPoolID         string
}

// Accessors for the parity fields. They are grouped here rather than beside the
// other accessors so the five fields draft 017 §4.1b adds stay legible as one
// set.

// GlobalPriority is the endpoint's order across providers: a lower value is
// tried first when several providers have a healthy account. Zero means unset,
// which is distinct from any explicit order.
func (e UpstreamEndpoint) GlobalPriority() int { return e.globalPriority }

// DefaultModel is the model used when a request names none. It is empty when the
// operator declared none.
func (e UpstreamEndpoint) DefaultModel() string { return e.defaultModel }

// ConsecutiveUseCount is the current run of served calls: it advances once per
// served call and resets on a success. It is the input a rotation or a
// stuck-account detector reads.
func (e UpstreamEndpoint) ConsecutiveUseCount() int { return e.consecutiveUseCount }

// ProxyPoolID names the stored proxy pool this endpoint egresses through, or ""
// when it dials directly.
func (e UpstreamEndpoint) ProxyPoolID() string { return e.proxyPoolID }

// LastError reports the last upstream error that was NOT a connectivity test
// result: the code, a scrubbed English message, and when it happened. All three
// are zero when the endpoint has not failed since its last success.
func (e UpstreamEndpoint) LastError() (code, message string, at *time.Time) {
	return e.lastErrorCode, e.lastErrorMessage, e.lastErrorAt
}

// SetRouting applies the operator-settable parity fields.
//
// The three are set together rather than one method each because they are one
// screen: an operator who changes an endpoint's routing changes its order, its
// default model, and its proxy in the same edit. A refused value leaves the
// aggregate untouched, so a partial write is not possible.
func (e *UpstreamEndpoint) SetRouting(globalPriority int, defaultModel, proxyPoolID string, now time.Time) error {
	if globalPriority < 0 {
		return NewValidationError("global_priority must not be negative")
	}
	model := strings.TrimSpace(defaultModel)
	if len(model) > maxDefaultModelLength {
		return NewValidationError("default_model must be at most 200 characters")
	}
	pool := strings.TrimSpace(proxyPoolID)
	if len(pool) > maxProxyPoolIDLength {
		return NewValidationError("proxy_pool_id must be at most 64 characters")
	}
	e.globalPriority = globalPriority
	e.defaultModel = model
	e.proxyPoolID = pool
	e.updatedAt = now
	return nil
}

// RecordUse counts one served call without touching health.
//
// It is separate from RecordUpstreamSuccess because the two answer different
// questions: this one asks "how long has this account been used in a row", the
// other "did the last call work". A served call that later failed still counts
// as a use, which is what makes the counter a rotation input rather than a health
// signal.
func (e *UpstreamEndpoint) RecordUse(now time.Time) {
	e.consecutiveUseCount++
	e.lastUsedAt = &now
}

// RecordUpstreamSuccess resets the run and clears the recorded error, because the
// endpoint has demonstrably recovered: leaving the error in place would show a
// failure the operator has already fixed.
func (e *UpstreamEndpoint) RecordUpstreamSuccess(now time.Time) {
	e.consecutiveUseCount = 0
	e.lastErrorCode = ""
	e.lastErrorMessage = ""
	e.lastErrorAt = nil
	e.lastUsedAt = &now
	e.updatedAt = now
}

// RecordUpstreamError stores the last error that was not a test result.
//
// The message is scrubbed before it is stored. It arrives from an upstream, which
// is exactly the text most likely to echo the credential it just rejected, and
// this column is rendered in the panel and may be logged (OWASP A09). A message
// that looks like it carries a secret is replaced rather than truncated: half a
// secret is still a secret, and the operator needs to know the credential was the
// problem, not to read the upstream's wording.
func (e *UpstreamEndpoint) RecordUpstreamError(code, message string, now time.Time) {
	stored := strings.TrimSpace(message)
	if len(stored) > maxLastErrorMessage {
		stored = stored[:maxLastErrorMessage]
	}
	if carriesCredentialMaterial(stored) {
		stored = credentialScrubbedMessage
	}
	at := now
	e.lastErrorCode = strings.TrimSpace(code)
	e.lastErrorMessage = stored
	e.lastErrorAt = &at
	e.updatedAt = now
}

// carriesCredentialMaterial reports whether a message looks like it carries a
// secret.
//
// The test is deliberately broad: it looks for the shapes a credential is written
// in (a bearer header, an api-key header, a vendor key prefix) rather than for a
// specific key, because the value is unknown at this point. A false positive
// costs an operator the upstream's exact wording; a false negative stores a live
// secret in a column the panel renders.
func carriesCredentialMaterial(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range []string{"bearer ", "x-api-key", "api-key:", "apikey=", "sk-", "key-", "token=", "authorization:"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
