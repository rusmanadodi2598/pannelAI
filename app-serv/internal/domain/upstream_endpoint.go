// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint.go
// @for       The UpstreamEndpoint aggregate root: one configured account at a
//
//	provider and the keys routing may spend for it (SPEC-API-001 §5, §7.5).
//
// @uses      internal/domain (ULID, AppError constructors, UpstreamKey).
// @reason    The endpoint is the mutation boundary for its keys (AGENTS.md
//
//	§2.2): "the endpoint must keep one usable credential" is a rule
//	about the collection, so it cannot live on a key, and the router
//	reads the ordering this type owns.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"time"
)

// UpstreamEndpoint is one configured account at a provider, owning 1..N keys.
// Fields are unexported on purpose (AGENTS.md §2.2): every transition and every
// key mutation goes through a method here.
type UpstreamEndpoint struct {
	id               string
	providerID       string
	label            string
	authType         UpstreamAuthType
	priority         int
	status           UpstreamEndpointStatus
	oauth            *OAuthCredential
	account          EndpointAccount
	testStatus       EndpointTestStatus
	rateLimitedUntil *time.Time
	lastUsedAt       *time.Time
	createdAt        time.Time
	updatedAt        time.Time
	keys             []UpstreamKey

	// The connection-parity fields (draft 017 §4.1b). Their rules live in
	// upstream_endpoint_parity.go.
	globalPriority      int
	defaultModel        string
	consecutiveUseCount int
	lastErrorCode       string
	lastErrorMessage    string
	lastErrorAt         *time.Time
	proxyPoolID         string
}

// NewUpstreamEndpoint is the only constructor for a new endpoint.
func NewUpstreamEndpoint(id, providerID, label string, authType UpstreamAuthType, priority int, now time.Time) (UpstreamEndpoint, error) {
	providerID = strings.TrimSpace(providerID)
	label = strings.TrimSpace(label)
	if id == "" {
		return UpstreamEndpoint{}, NewValidationError("endpoint id is required")
	}
	if providerID == "" {
		return UpstreamEndpoint{}, NewValidationError("provider_id is required")
	}
	if label == "" {
		return UpstreamEndpoint{}, NewValidationError("endpoint label is required")
	}
	if len(label) > 120 {
		return UpstreamEndpoint{}, NewValidationError("endpoint label must be at most 120 characters")
	}
	if _, err := ParseUpstreamAuthType(string(authType)); err != nil {
		return UpstreamEndpoint{}, err
	}
	if priority < 1 {
		return UpstreamEndpoint{}, NewValidationError("priority must be at least 1")
	}
	return UpstreamEndpoint{
		id:         id,
		providerID: providerID,
		label:      label,
		authType:   authType,
		priority:   priority,
		status:     UpstreamEndpointActive,
		createdAt:  now,
		updatedAt:  now,
		keys:       make([]UpstreamKey, 0, 1),
	}, nil
}

// RehydrateUpstreamEndpoint rebuilds a stored row with its keys. It is for the
// repository load path only.
func RehydrateUpstreamEndpoint(
	id, providerID, label string,
	authType UpstreamAuthType,
	priority int,
	status UpstreamEndpointStatus,
	oauth *OAuthCredential,
	account EndpointAccount,
	testStatus EndpointTestStatus,
	rateLimitedUntil, lastUsedAt *time.Time,
	createdAt, updatedAt time.Time,
	keys []UpstreamKey,
	parity EndpointParity,
) UpstreamEndpoint {
	owned := make([]UpstreamKey, len(keys))
	copy(owned, keys)
	return UpstreamEndpoint{
		id: id, providerID: providerID, label: label, authType: authType,
		priority: priority, status: status, oauth: oauth, account: account,
		testStatus: testStatus, rateLimitedUntil: rateLimitedUntil, lastUsedAt: lastUsedAt,
		createdAt: createdAt, updatedAt: updatedAt, keys: owned,
		globalPriority:      parity.GlobalPriority,
		defaultModel:        parity.DefaultModel,
		consecutiveUseCount: parity.ConsecutiveUseCount,
		lastErrorCode:       parity.LastErrorCode,
		lastErrorMessage:    parity.LastErrorMessage,
		lastErrorAt:         parity.LastErrorAt,
		proxyPoolID:         parity.ProxyPoolID,
	}
}

// Accessors expose state without allowing mutation.
func (e UpstreamEndpoint) ID() string                     { return e.id }
func (e UpstreamEndpoint) ProviderID() string             { return e.providerID }
func (e UpstreamEndpoint) Label() string                  { return e.label }
func (e UpstreamEndpoint) AuthType() UpstreamAuthType     { return e.authType }
func (e UpstreamEndpoint) Priority() int                  { return e.priority }
func (e UpstreamEndpoint) Status() UpstreamEndpointStatus { return e.status }
func (e UpstreamEndpoint) OAuth() *OAuthCredential        { return e.oauth }
func (e UpstreamEndpoint) Account() EndpointAccount       { return e.account }
func (e UpstreamEndpoint) TestStatus() EndpointTestStatus { return e.testStatus }
func (e UpstreamEndpoint) RateLimitedUntil() *time.Time   { return e.rateLimitedUntil }
func (e UpstreamEndpoint) LastUsedAt() *time.Time         { return e.lastUsedAt }
func (e UpstreamEndpoint) CreatedAt() time.Time           { return e.createdAt }
func (e UpstreamEndpoint) UpdatedAt() time.Time           { return e.updatedAt }

func (e UpstreamEndpoint) Available(now time.Time) bool {
	if e.status != UpstreamEndpointActive {
		return false
	}
	if e.rateLimitedUntil != nil && now.Before(*e.rateLimitedUntil) {
		return false
	}
	// An endpoint that needs a credential but has no usable one cannot serve a
	// request; reporting it unavailable is more useful than letting the router
	// discover it one call later.
	if e.authType == UpstreamAuthAPIKey {
		_, ok := e.NextKey(now)
		return ok
	}
	return true
}

// Update applies a PATCH to the endpoint's own fields. The status is parsed from
// the wire value, and children are unaffected.
func (e *UpstreamEndpoint) Update(label string, priority int, status string, now time.Time) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return NewValidationError("endpoint label is required")
	}
	if len(label) > 120 {
		return NewValidationError("endpoint label must be at most 120 characters")
	}
	if priority < 1 {
		return NewValidationError("priority must be at least 1")
	}
	if status != "" {
		parsed, err := ParseUpstreamEndpointStatus(status)
		if err != nil {
			return err
		}
		e.status = parsed
		if parsed == UpstreamEndpointDisabled {
			// A disabled endpoint is not in a backoff, and its keys are not
			// being spent, so the window is cleared rather than left to show a
			// countdown for something switched off.
			e.rateLimitedUntil = nil
		}
	}
	e.label = label
	e.priority = priority
	e.updatedAt = now
	return nil
}

// SetAccount replaces the identifying fields of the account.
func (e *UpstreamEndpoint) SetAccount(account EndpointAccount, now time.Time) {
	e.account = account
	e.updatedAt = now
}

// SetOAuth replaces the token set an OAuth endpoint carries.
func (e *UpstreamEndpoint) SetOAuth(credential *OAuthCredential, now time.Time) {
	e.oauth = credential
	e.updatedAt = now
}

// RecordTest stores the outcome of a connectivity test.
func (e *UpstreamEndpoint) RecordTest(state string, latencyMS int, message string, now time.Time) {
	checked := now
	e.testStatus = EndpointTestStatus{State: state, LatencyMS: latencyMS, CheckedAt: &checked, Message: message}
	e.updatedAt = now
}

// MarkRateLimited puts the whole endpoint in a backoff window, which is what a
// provider-wide quota rejection means: every key of this account is affected.
func (e *UpstreamEndpoint) MarkRateLimited(until time.Time) {
	window := until
	e.rateLimitedUntil = &window
}

// ClearRateLimit lifts the endpoint-wide window after a served request.
func (e *UpstreamEndpoint) ClearRateLimit(now time.Time) {
	e.rateLimitedUntil = nil
	e.lastUsedAt = &now
	e.updatedAt = now
}
