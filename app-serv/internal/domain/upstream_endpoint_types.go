// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_types.go
// @for       The endpoint vocabulary: status, auth type, and the credential the entity carries for an OAuth account.
// @uses      internal/domain (error constructors), strings, time.
// @reason    SPEC-API-001 §5 names these as the shared language between the panel and the router, so they are declared once here rather than as DTO fields; keeping the parse functions beside the types is what keeps a wire value validated before it reaches an entity.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-17
package domain

import (
	"strings"
	"time"
)

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
// an accidental log of its state cannot leak a credential. There is no way to
// write one of its fields from outside; a rotation is a method on the credential.
type OAuthCredential struct {
	accessTokenEncrypted  string
	refreshTokenEncrypted string
	expiresAt             *time.Time
	scopes                []string
	projectID             string
	accountID             string
	accountEmail          Email
	lastRefreshAt         *time.Time
}

// OAuthCredentialInput is what a caller presents for a stored credential. Every
// token field must already be ciphertext: the aggregate never sees plaintext.
type OAuthCredentialInput struct {
	AccessTokenEncrypted  string
	RefreshTokenEncrypted string
	ExpiresAt             *time.Time
	Scopes                []string
	ProjectID             string
	AccountID             string
	AccountEmail          string
	LastRefreshAt         *time.Time
}

// NewOAuthCredential builds the token set an OAuth endpoint carries and refuses
// one with no access ciphertext: a credential that stores nothing still reads as
// present, and the request it produces reaches the upstream unauthenticated.
func NewOAuthCredential(input OAuthCredentialInput) (*OAuthCredential, error) {
	if strings.TrimSpace(input.AccessTokenEncrypted) == "" {
		return nil, NewValidationError("an oauth credential needs an encrypted access token")
	}
	email, err := ParseEmail(input.AccountEmail)
	if err != nil {
		return nil, err
	}
	return &OAuthCredential{
		accessTokenEncrypted:  strings.TrimSpace(input.AccessTokenEncrypted),
		refreshTokenEncrypted: strings.TrimSpace(input.RefreshTokenEncrypted),
		expiresAt:             input.ExpiresAt,
		scopes:                append([]string(nil), input.Scopes...),
		projectID:             strings.TrimSpace(input.ProjectID),
		accountID:             strings.TrimSpace(input.AccountID),
		accountEmail:          email,
		lastRefreshAt:         input.LastRefreshAt,
	}, nil
}

// RehydrateOAuthCredential rebuilds a stored credential for the repository load
// path, without re-validating material the row already holds.
func RehydrateOAuthCredential(input OAuthCredentialInput) *OAuthCredential {
	return &OAuthCredential{
		accessTokenEncrypted:  input.AccessTokenEncrypted,
		refreshTokenEncrypted: input.RefreshTokenEncrypted,
		expiresAt:             input.ExpiresAt,
		scopes:                append([]string(nil), input.Scopes...),
		projectID:             input.ProjectID,
		accountID:             input.AccountID,
		accountEmail:          Email{value: NormalizeEmail(input.AccountEmail)},
		lastRefreshAt:         input.LastRefreshAt,
	}
}

func (c *OAuthCredential) AccessTokenEncrypted() string  { return c.accessTokenEncrypted }
func (c *OAuthCredential) RefreshTokenEncrypted() string { return c.refreshTokenEncrypted }
func (c *OAuthCredential) ExpiresAt() *time.Time         { return c.expiresAt }
func (c *OAuthCredential) Scopes() []string              { return append([]string(nil), c.scopes...) }
func (c *OAuthCredential) ProjectID() string             { return c.projectID }
func (c *OAuthCredential) AccountID() string             { return c.accountID }
func (c *OAuthCredential) AccountEmail() Email           { return c.accountEmail }
func (c *OAuthCredential) LastRefreshAt() *time.Time     { return c.lastRefreshAt }

// HasRefreshToken reports that the account can be renewed rather than re-login.
func (c *OAuthCredential) HasRefreshToken() bool {
	return strings.TrimSpace(c.refreshTokenEncrypted) != ""
}

// Rotated returns the credential an account holds after a successful refresh
// grant: new access ciphertext, the new refresh ciphertext when the vendor issued
// one, and the stamp of when it happened. A blank refresh keeps the token the
// account already holds, because some providers renew the access token only and
// dropping the refresh material would end the account's ability to renew at all.
// A grant that reports no lifetime keeps the expiry held for the same reason.
//
// It returns a value rather than mutating one: the aggregate hands out this
// credential to read, and a caller that wrote through that pointer would change
// the account's state without going through the aggregate at all.
func (c OAuthCredential) Rotated(accessToken, refreshToken string, expiresAt *time.Time, now time.Time) (OAuthCredential, error) {
	if strings.TrimSpace(accessToken) == "" {
		return OAuthCredential{}, NewValidationError("a rotated credential needs an encrypted access token")
	}
	rotated := c
	rotated.accessTokenEncrypted = strings.TrimSpace(accessToken)
	if strings.TrimSpace(refreshToken) != "" {
		rotated.refreshTokenEncrypted = strings.TrimSpace(refreshToken)
	}
	if expiresAt != nil {
		rotated.expiresAt = expiresAt
	}
	rotated.scopes = append([]string(nil), c.scopes...)
	stamped := now.UTC()
	rotated.lastRefreshAt = &stamped
	return rotated, nil
}
