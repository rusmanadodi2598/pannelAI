// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_credential.go
// @for       The credential one upstream account presents, and the auth family it
//
//	belongs to.
//
// @uses      (none).
// @reason    A provider may read a different header per credential family, so
//
//	choosing the header and choosing the value separately is how an OAuth
//	token ends up in a static-key header. Reducing every family to one
//	shape is what lets the core never branch on which one is in use, and
//	it lives apart from the Plugin interface because the interface is the
//	seam while this is the value that travels through it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-21
package provider

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
