// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_service_account.go
// @for       Exchanging a service-account credential for the bearer its media kind presents, before the call is built.
// @uses      internal/dataplane, internal/provider, internal/registry, context, strings.
// @reason    A service-account kind stores the vendor's JSON like a key but authenticates with a short-lived token, so the exchange sits in the credential seam: every route that binds an account runs through it, and a kind that is not service_account is untouched. Refusing without a minter is what keeps the stored private key from being presented as a bearer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// mintCredential exchanges a service-account credential for the access token
// its kind presents, and passes every other credential through untouched.
//
// A service_account kind's block declares a bearer, so the placement is the
// same as any other token's; only the value differs, and it is minted here
// rather than at the route so every phase of a job (a creation and its polls)
// exchanges the same way.
func (s *MediaCallService) mintCredential(ctx context.Context, call MediaCall, credential provider.Credential) (provider.Credential, error) {
	if call.Media.AuthType != registry.AuthServiceAccount {
		return credential, nil
	}
	if s.tokens == nil {
		return provider.Credential{}, dataplane.InternalError(
			"the media service has no service-account token minter wired", nil)
	}
	_, secret, err := credential.Secret()
	if err != nil {
		return provider.Credential{}, dataplane.InternalError("the media account's credential could not be resolved", err)
	}
	if strings.TrimSpace(secret) == "" {
		return provider.Credential{}, dataplane.ValidationError("the service-account endpoint has no stored credential")
	}
	token, err := s.tokens.AccessToken(ctx, secret)
	if err != nil {
		return provider.Credential{}, dataplane.UpstreamRejected(502,
			"the service account could not mint an access token: "+err.Error())
	}
	return provider.OAuthToken(credential.EndpointID(), credential.KeyID(), token), nil
}
