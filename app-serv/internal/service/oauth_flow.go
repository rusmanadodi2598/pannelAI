// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow.go
// @for       Starting a provider authorization: state minting, PKCE, and the
//
//	authorize URL (SPEC-API-001 §7.4 POST .../oauth/start).
//
// @uses      crypto/rand, encoding/base64, encoding/json, net/url, strings,
//
//	time, internal/domain, internal/registry, internal/repository.
//
// @reason    §4 makes `state` a single-use, ten-minute replay guard and the
//
//	registry the only source of authorize endpoints, scopes, and
//	the PKCE method, so this file mints the two random values,
//	stages the flow's private context, and renders the URL the
//	browser follows. Every value it emits is derived from the
//	provider entry and CSPRNG output; nothing is per-provider code.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// oauthStateTTL is the §4 single-use window: ten minutes between an operator
// starting a flow and the provider calling back.
const oauthStateTTL = 10 * time.Minute

// OAuthAccountStore is the slice of endpoint persistence the OAuth flow needs:
// reading a provider's accounts, matching an account identity, and writing the
// token set a connect or refresh produced. The full EndpointStore satisfies it;
// declaring only these methods keeps the handler tests' double small and stops
// the flow from reaching key-scoped writes it has no business making.
type OAuthAccountStore interface {
	Create(ctx context.Context, endpoint domain.UpstreamEndpoint) error
	List(ctx context.Context, filter repository.EndpointFilter, q repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error)
	GetByID(ctx context.Context, id string) (domain.UpstreamEndpoint, error)
	Update(ctx context.Context, endpoint domain.UpstreamEndpoint) error
	FindOAuthEndpoint(ctx context.Context, providerID, email, workspaceID string) (string, error)
}

// OAuthFlowService implements the §7.4 OAuth routes.
type OAuthFlowService struct {
	store  OAuthAccountStore
	index  ProviderIndex
	states repository.OAuthStateStore
	tokens OAuthTokenClient
	sealer CredentialSealer
	clock  func() time.Time
}

// OAuthFlowDeps holds the collaborators the flow is assembled from. Every one
// is a seam: registry lookup, account storage, state staging, the token
// endpoint client, and the credential sealer.
type OAuthFlowDeps struct {
	Index  ProviderIndex
	Store  OAuthAccountStore
	States repository.OAuthStateStore
	Tokens OAuthTokenClient
	Sealer CredentialSealer
}

// NewOAuthFlowService validates deps and returns a ready service.
func NewOAuthFlowService(deps OAuthFlowDeps) (*OAuthFlowService, error) {
	if deps.Index == nil {
		return nil, domain.NewValidationError("provider index is required")
	}
	if deps.Store == nil {
		return nil, domain.NewValidationError("oauth account store is required")
	}
	if deps.States == nil {
		return nil, domain.NewValidationError("oauth state store is required")
	}
	if deps.Tokens == nil {
		return nil, domain.NewValidationError("oauth token client is required")
	}
	if deps.Sealer == nil {
		return nil, domain.NewValidationError("credential sealer is required")
	}
	return &OAuthFlowService{
		index: deps.Index, store: deps.Store, states: deps.States,
		tokens: deps.Tokens, sealer: deps.Sealer, clock: time.Now,
	}, nil
}

// OAuthStartInput is one start request: the provider, an optional explicit
// redirect, and the public base URL the panel answers on.
type OAuthStartInput struct {
	ProviderID  string
	RedirectURI string
	BaseURL     string
}

// OAuthStart is the answer the panel turns into a redirect.
type OAuthStart struct {
	AuthorizeURL string
	State        string
}

// Start mints the state and (when the provider demands S256) the PKCE pair,
// stages the flow's private context, and renders the authorize URL.
func (s *OAuthFlowService) Start(ctx context.Context, in OAuthStartInput) (OAuthStart, error) {
	provider, oauth, err := s.requireCodeFlow(in.ProviderID)
	if err != nil {
		return OAuthStart{}, err
	}

	redirect := strings.TrimSpace(in.RedirectURI)
	if redirect == "" {
		if strings.TrimSpace(in.BaseURL) == "" {
			return OAuthStart{}, domain.NewValidationError("a redirect URI is required: pass redirect_uri or serve behind a public base URL")
		}
		redirect = callbackURLFor(strings.TrimSuffix(in.BaseURL, "/"), provider.ID)
	}
	if err := validateRedirectURI(redirect); err != nil {
		return OAuthStart{}, err
	}

	state, err := randomToken(32)
	if err != nil {
		return OAuthStart{}, domain.NewInternalError("the oauth state could not be generated")
	}
	verifier := ""
	if strings.EqualFold(oauth.CodeChallenge, "S256") {
		if verifier, err = randomToken(32); err != nil {
			return OAuthStart{}, domain.NewInternalError("the PKCE verifier could not be generated")
		}
	}

	payload, err := json.Marshal(oauthStatePayload{
		ProviderID: provider.ID, Verifier: verifier,
		RedirectURI: redirect, Origin: originFor(in.BaseURL, redirect),
	})
	if err != nil {
		return OAuthStart{}, domain.NewInternalError("the oauth state could not be encoded")
	}
	if err := s.states.Stage(ctx, state, payload, oauthStateTTL); err != nil {
		if err == repository.ErrStateAlreadyStaged {
			return OAuthStart{}, domain.NewConflictError("an oauth state is already in flight")
		}
		return OAuthStart{}, err
	}

	authorize, err := buildAuthorizeURL(oauth, redirect, state, verifier)
	if err != nil {
		return OAuthStart{}, err
	}
	return OAuthStart{AuthorizeURL: authorize, State: state}, nil
}

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
