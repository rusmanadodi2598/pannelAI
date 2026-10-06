// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow_authorize.go
// @for       The authorize URL a round starts on, and the parameters it is allowed to carry.
// @uses      crypto/rand, crypto/sha256, encoding/base64, net/url, strings, internal/domain, internal/registry.
// @reason    Every one of these is a rule about what leaves the process toward a provider: the PKCE pairing RFC 7636 defines, the alphabet that survives a query parameter, the absolute http(s) redirect a browser may be sent to, and the core parameters a vendor extra may add to but never override. They are apart from the round's orchestration so a provider quirk in the URL is a one-file change.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-06
package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// coreAuthorizeParams are the parameters this service owns. A provider's
// extra_params may add to them but never replace them, because a vendor extra
// that overrode response_type would break the flow the state was staged for.
var coreAuthorizeParams = map[string]struct{}{
	"response_type": {}, "client_id": {}, "redirect_uri": {}, "state": {},
	"scope": {}, "code_challenge": {}, "code_challenge_method": {},
}

// buildAuthorizeURL renders the provider's authorize endpoint with the query
// the code flow requires. Scopes join with a space (OAuth's form encoding) and
// the PKCE pair rides along only when a verifier exists.
func buildAuthorizeURL(oauth *registry.OAuth, redirect, state, verifier string) (string, error) {
	parsed, err := url.Parse(oauth.AuthorizeURL)
	if err != nil {
		return "", domain.NewInternalError("the provider's authorize URL is malformed")
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", oauth.ClientID)
	query.Set("redirect_uri", redirect)
	query.Set("state", state)
	if scopes := oauth.ScopeList(); len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, " "))
	}
	if verifier != "" {
		query.Set("code_challenge", pkceChallenge(verifier))
		query.Set("code_challenge_method", "S256")
	}
	for name, value := range oauth.ExtraParams {
		if _, core := coreAuthorizeParams[name]; !core {
			query.Set(name, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// pkceChallenge computes the S256 challenge of a verifier: unpadded base64url
// of the verifier's SHA-256, per RFC 7636.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randomToken returns n CSPRNG bytes as unpadded base64url, the alphabet both
// the state and the PKCE verifier use because it survives a query parameter
// without further encoding.
func randomToken(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// validateRedirectURI admits only absolute http(s) redirects. Anything else,
// including javascript: and file:, is refused at the boundary rather than
// stored in a state a browser will later be sent to.
func validateRedirectURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return domain.NewValidationError("redirect_uri must be an absolute http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return domain.NewValidationError("redirect_uri must be an absolute http(s) URL")
	}
	return nil
}
