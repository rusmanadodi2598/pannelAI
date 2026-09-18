// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_types.go
// @for       The endpoint vocabulary: status, auth type, and the account and
//
//	credential value objects an endpoint carries.
//
// @uses      internal/domain (error constructors), time.
// @reason    SPEC-API-001 §5 names these as the shared language between the
//
//	panel and the router, so they are declared once here rather than
//	as DTO fields; keeping the parse functions beside the types is what
//	keeps a wire value validated before it reaches an entity.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import "time"

// UpstreamEndpointStatus is the lifecycle state of an upstream endpoint.
type UpstreamEndpointStatus string

const (
	UpstreamEndpointActive   UpstreamEndpointStatus = "active"
	UpstreamEndpointDisabled UpstreamEndpointStatus = "disabled"
	UpstreamEndpointError    UpstreamEndpointStatus = "error"
)

// UpstreamAuthType is how an endpoint authenticates to its provider. Every
// provider falls into one of these three, plus the case where it accepts both a
// key and a flow, which is expressed by the provider's auth modes rather than by
// a fourth value here: one endpoint still authenticates exactly one way.
type UpstreamAuthType string

const (
	UpstreamAuthAPIKey UpstreamAuthType = "api_key"
	UpstreamAuthOAuth  UpstreamAuthType = "oauth"
	UpstreamAuthNone   UpstreamAuthType = "no_auth"
)

// ParseUpstreamAuthType validates an auth type before it reaches the entity.
func ParseUpstreamAuthType(s string) (UpstreamAuthType, error) {
	authType := UpstreamAuthType(s)
	switch authType {
	case UpstreamAuthAPIKey, UpstreamAuthOAuth, UpstreamAuthNone:
		return authType, nil
	default:
		return "", NewValidationError("invalid auth_type: " + s)
	}
}

// ParseUpstreamEndpointStatus validates an endpoint status from the wire.
// `error` is deliberately not settable: health tracking owns it, so a client
// cannot mark an endpoint the router still considers usable.
func ParseUpstreamEndpointStatus(s string) (UpstreamEndpointStatus, error) {
	status := UpstreamEndpointStatus(s)
	switch status {
	case UpstreamEndpointActive, UpstreamEndpointDisabled:
		return status, nil
	default:
		return "", NewValidationError("invalid status: " + s)
	}
}

// OAuthCredential is the token set an OAuth endpoint carries. Every field holds
// ciphertext rather than token material: the aggregate never sees plaintext, so
// an accidental log of its state cannot leak a credential.
type OAuthCredential struct {
	AccessTokenEncrypted  string
	RefreshTokenEncrypted string
	ExpiresAt             *time.Time
	Scopes                []string
	ProjectID             string
	AccountID             string
	AccountEmail          string
	LastRefreshAt         *time.Time
}

// EndpointAccount is the non-secret identity an endpoint presents. It is what
// distinguishes two accounts of the same provider in the panel, and it is what
// makes a re-import of the same account detectable.
type EndpointAccount struct {
	Name        string
	Email       string
	MachineID   string
	WorkspaceID string
}

// EndpointTestStatus is the outcome of the last connectivity test.
type EndpointTestStatus struct {
	State     string
	LatencyMS int
	CheckedAt *time.Time
	Message   string
}

// Test state values. A test either answered or it did not, so there are two.
const (
	EndpointTestOK   = "ok"
	EndpointTestFail = "fail"
)
