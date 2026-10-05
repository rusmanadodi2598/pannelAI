// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_credential.go
// @for       The credential one upstream account presents, and the auth family it
//
//	belongs to.
//
// @uses      fmt, strings.
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
// @stability stable
// @since     2026-09-21
package provider

import (
	"fmt"
	"strings"
)

// Family is which kind of credential an account holds, decided once from the
// account's own auth type: a provider may read a different header per family, so
// choosing the header and the value separately is how an OAuth token ends up in a
// static-key header. It is exported because the caller assembling a credential
// lives in the data plane, not in this package.
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

// Credential is what one upstream account presents to its provider: the single
// shape every auth family reduces to, so the core never branches on which family
// is in use. Exactly ONE of the two secrets may be held, and the fields are
// unexported so that is a property of the value rather than a paragraph a caller
// can read and ignore: every way in is a constructor, Secret is the only way out.
// Holding both gives a provider that reads a different header per family two
// contradictory signals. The plaintext exists for one request only, assembled from
// encrypted storage by the caller and never persisted here.
type Credential struct {
	// endpointID and keyID identify the account and, for a multi-key endpoint,
	// the exact key that was picked. They are what accounting records and what
	// a failure is attributed to.
	endpointID string
	keyID      string

	apiKey      string
	accessToken string

	// family states which credential kind this account presents. FamilyUnset
	// means no credential material is present, which is the correct state for a
	// provider that needs none.
	family Family

	account   string
	projectID string

	// metadata carries the provider-specific, non-secret values a connector
	// needs: a region, a client version, an editor identity. It is a map rather
	// than a typed struct because only the owning connector interprets it, and
	// a shared struct would accumulate every provider's fields. The keys a caller
	// may rely on are declared beside MetadataMachineID rather than invented per
	// connector, so a writer and its reader cannot spell them differently.
	metadata map[string]string
}

// CredentialInput is the full shape the data plane assembles for one call. The
// single-family constructors below are the right choice for every other caller.
type CredentialInput struct {
	EndpointID  string
	KeyID       string
	APIKey      string
	AccessToken string
	Family      Family
	Account     string
	ProjectID   string
	Metadata    map[string]string
}

// NewCredential builds the one shape from the data plane's assembly and refuses a
// credential holding both secrets, so the ambiguity is caught where the value is
// made rather than at the first header that happens to be checked.
func NewCredential(input CredentialInput) (Credential, error) {
	if strings.TrimSpace(input.APIKey) != "" && strings.TrimSpace(input.AccessToken) != "" {
		return Credential{}, ambiguousCredentialError(input.EndpointID, input.KeyID)
	}
	metadata := make(map[string]string, len(input.Metadata))
	for key, value := range input.Metadata {
		metadata[key] = value
	}
	return Credential{
		endpointID:  input.EndpointID,
		keyID:       input.KeyID,
		apiKey:      strings.TrimSpace(input.APIKey),
		accessToken: strings.TrimSpace(input.AccessToken),
		family:      input.Family,
		account:     input.Account,
		projectID:   input.ProjectID,
		metadata:    metadata,
	}, nil
}

// MetadataMachineID names the account's persisted device-fingerprint id. A
// provider whose signed requests replay it (Qoder's COSY layer) reads it from a
// device login's stored account; nothing else in the Credential describes it,
// because it is neither a secret nor a workspace.
const MetadataMachineID = "machine_id"

// StaticKey builds a credential presenting a long-lived key.
func StaticKey(endpointID, keyID, value string) Credential {
	return Credential{endpointID: endpointID, keyID: keyID, apiKey: value, family: FamilyStaticKey}
}

// OAuthToken builds a credential presenting a token from an authorization flow.
func OAuthToken(endpointID, keyID, value string) Credential {
	return Credential{endpointID: endpointID, keyID: keyID, accessToken: value, family: FamilyOAuth}
}

// NoCredential builds a credential for a provider that needs none.
func NoCredential(endpointID string) Credential {
	return Credential{endpointID: endpointID}
}

func (c Credential) EndpointID() string { return c.endpointID }
func (c Credential) KeyID() string      { return c.keyID }
func (c Credential) Account() string    { return c.account }
func (c Credential) ProjectID() string  { return c.projectID }
func (c Credential) FamilyKind() Family { return c.family }

// MetadataValue reads one provider-specific non-secret value by the key its
// writer and its reader agree on.
func (c Credential) MetadataValue(key string) string { return c.metadata[key] }

// Secret reports which credential this account presents and its value, and refuses
// an account holding both. An explicitly declared family wins; otherwise the single
// populated field decides. Both secrets populated is refused rather than resolved:
// a provider that reads a different header per family has no correct answer for
// such an account, and choosing one silently would route a request under a
// credential its owner never meant to present.
func (c Credential) Secret() (Family, string, error) {
	if c.apiKey != "" && c.accessToken != "" {
		return FamilyUnset, "", ambiguousCredentialError(c.endpointID, c.keyID)
	}
	switch c.family {
	case FamilyStaticKey:
		return FamilyStaticKey, c.apiKey, nil
	case FamilyOAuth:
		return FamilyOAuth, c.accessToken, nil
	}
	switch {
	case c.accessToken != "":
		return FamilyOAuth, c.accessToken, nil
	case c.apiKey != "":
		return FamilyStaticKey, c.apiKey, nil
	default:
		return FamilyUnset, "", nil
	}
}

// HasCredential reports whether any credential material is present. A
// credential-free provider returns false and the core must not treat that as an
// error.
func (c Credential) HasCredential() bool {
	_, value, err := c.Secret()
	return err == nil && value != ""
}

// String names the account without naming the credential. The plaintext escapes
// through a debug print, a test failure dumping the struct, or a `%+v` in a log
// line someone adds later, so this prints the identity fields and redacts the
// secrets.
func (c Credential) String() string {
	return "provider.Credential{EndpointID: " + c.endpointID +
		", KeyID: " + c.keyID +
		", Account: " + c.account +
		", ProjectID: " + c.projectID +
		", APIKey: " + redactedWhenSet(c.apiKey) +
		", AccessToken: " + redactedWhenSet(c.accessToken) + "}"
}

func redactedWhenSet(value string) string {
	if value == "" {
		return "unset"
	}
	return "[redacted]"
}

func ambiguousCredentialError(endpointID, keyID string) error {
	return fmt.Errorf("account %s key %s: both a static key and an access token are set; exactly one may be",
		endpointID, keyID)
}
