// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/credential.go
// @for       Assembling the credential an upstream call presents, from the
//
//	endpoint and the key selection picked.
//
// @uses      internal/domain, internal/provider.
// @reason    SPEC-API-001 §6 stores every upstream credential as AES-GCM
//
//	ciphertext, so the plaintext exists only between opening the stored
//	value and the outbound request. Keeping that window in one function
//	is what makes it auditable: the plaintext is returned to the caller
//	for one call and never placed on the selection.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// credential assembles what the upstream call presents, decrypting the sealed
// value exactly once for this request.
//
// The three auth families the domain defines decide the shape: a no_auth
// endpoint sends nothing, an oauth endpoint presents its access token, and an
// api_key endpoint presents the selected key's value. The endpoint's own auth
// type always wins over the provider's default, because one account
// authenticates exactly one way (SPEC-API-001 §8.1).
func (s *Selector) credential(endpoint domain.UpstreamEndpoint, key domain.UpstreamKey) (provider.Credential, error) {
	credential := provider.Credential{
		EndpointID: endpoint.ID(),
		KeyID:      key.ID(),
		Account:    endpoint.Account().Email,
		ProjectID:  endpoint.Account().WorkspaceID,
	}
	switch endpoint.AuthType() {
	case domain.UpstreamAuthNone:
		return credential, nil
	case domain.UpstreamAuthOAuth:
		oauth := endpoint.OAuth()
		if oauth == nil || oauth.AccessTokenEncrypted == "" {
			return provider.Credential{}, internalError("the selected endpoint has no stored token", nil)
		}
		token, err := s.open(oauth.AccessTokenEncrypted)
		if err != nil {
			return provider.Credential{}, err
		}
		credential.AccessToken = token
		if oauth.ProjectID != "" {
			credential.ProjectID = oauth.ProjectID
		}
		return credential, nil
	default:
		value, err := s.open(key.EncryptedValue())
		if err != nil {
			return provider.Credential{}, err
		}
		credential.APIKey = value
		return credential, nil
	}
}

// open decrypts a stored secret through the opener the composition root wired.
//
// The stored value is already ciphertext the aggregate never reads, so an
// unreadable one is an internal failure rather than a client-visible error: the
// client cannot act on a stale key, and reporting the opener's message would
// describe the gateway's storage format to a CLI tool.
func (s *Selector) open(sealed string) (string, error) {
	if s.opener == nil {
		return "", internalError("upstream credentials cannot be read", nil)
	}
	plaintext, err := s.opener.Open(sealed)
	if err != nil {
		return "", internalError("upstream credential could not be read", err)
	}
	return plaintext, nil
}

// CredentialForEndpoint builds the credential outside a selection, for a caller
// that already knows which account it targets (a connectivity test). It runs the
// same three-branch rule as selection, so a test exercises exactly what routing
// does rather than a second interpretation of the auth types.
func CredentialForEndpoint(endpoint domain.UpstreamEndpoint, key domain.UpstreamKey, opener SecretOpener) (provider.Credential, error) {
	selector := &Selector{opener: opener}
	return selector.credential(endpoint, key)
}
