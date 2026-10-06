// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_flow.go
// @for       Starting a provider authorization: the single-use state, its staged context, and the answer the panel follows (SPEC-API-001 §7.4 POST .../oauth/start).
// @uses      context, encoding/json, strings, time, internal/domain, internal/repository, internal/service/oauthhttp.
// @reason    §4 makes `state` a single-use, ten-minute replay guard and the registry the only source of authorize endpoints, scopes, and the PKCE method, so this file mints the two random values and stages the flow's private context; the URL they travel in is oauth_flow_authorize.go. Every value it emits is derived from the provider entry and CSPRNG output; nothing is per-provider code.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/oauthhttp"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
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
	UpdateIfUnchanged(ctx context.Context, endpoint domain.UpstreamEndpoint, loaded domain.OAuthCredential) error
	FindOAuthEndpoint(ctx context.Context, providerID, email, workspaceID string) (string, error)
}

// OAuthFlowService implements the §7.4 OAuth routes.
type OAuthFlowService struct {
	store  OAuthAccountStore
	index  ProviderIndex
	states repository.OAuthStateStore
	tokens oauthhttp.OAuthTokenClient
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
	Tokens oauthhttp.OAuthTokenClient
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
