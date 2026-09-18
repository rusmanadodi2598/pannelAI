// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_oauth_bulk.go
// @for       Importing already-obtained OAuth credentials as endpoints
//
//	(SPEC-API-001 §7.5, §8.1).
//
// @uses      internal/domain, context, errors, strconv, strings, time.
// @reason    The import path exists for accounts obtained on a machine with no
//
//	browser callback, so there is no authorization flow to run — only
//	tokens to seal and an account identity to match. It is separate from
//	endpoint_bulk.go because the two batches share the all-or-nothing
//	rule but nothing else, and because AGENTS.md §1.1 caps a file at 250
//	lines.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// OAuthAccountInput is one already-obtained token set to import (§7.5). The tokens
// are plaintext on entry and are sealed here; nothing above this layer sees them
// again.
type OAuthAccountInput struct {
	Label        string
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
	Scopes       []string
	Account      domain.EndpointAccount
}

// OAuthImportResult is one imported account's outcome: its identity and a hint
// only, never a token (§8.1).
type OAuthImportResult struct {
	Index     int
	Endpoint  domain.UpstreamEndpoint
	Updated   bool
	TokenHint string
}

// BulkImportOAuth imports already-obtained OAuth credentials as endpoints.
//
// An account identity already staged is updated in place rather than duplicated,
// because §8.1 makes a re-import an update: importing the same account twice must
// not leave two endpoints competing for one token set.
func (s *EndpointService) BulkImportOAuth(ctx context.Context, providerID string, accounts []OAuthAccountInput) ([]OAuthImportResult, error) {
	if len(accounts) == 0 {
		return nil, domain.NewValidationError("accounts is required")
	}
	if len(accounts) > maxBatchRows {
		return nil, domain.NewValidationError("a batch may hold at most 50 accounts")
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, domain.NewValidationError("provider_id is required")
	}
	if _, ok := s.index.Provider(providerID); !ok {
		return nil, domain.NewValidationError("unknown provider_id: " + providerID)
	}

	results := make([]OAuthImportResult, 0, len(accounts))
	for i, account := range accounts {
		result, err := s.importOneOAuth(ctx, providerID, account)
		if err != nil {
			return nil, rowError(i, err)
		}
		result.Index = i
		results = append(results, result)
	}
	return results, nil
}

// importOneOAuth seals one credential set and writes it onto the endpoint that
// already stands for the account, or onto a new one.
func (s *EndpointService) importOneOAuth(ctx context.Context, providerID string, account OAuthAccountInput) (OAuthImportResult, error) {
	accessToken := strings.TrimSpace(account.AccessToken)
	if accessToken == "" {
		return OAuthImportResult{}, domain.NewValidationError("access_token is required")
	}
	now := s.clock()
	credential, err := s.sealOAuthCredential(account, now)
	if err != nil {
		return OAuthImportResult{}, err
	}

	existingID, err := s.store.FindOAuthEndpoint(ctx, providerID, account.Account.Email, account.Account.WorkspaceID)
	if err != nil && !errors.Is(err, domain.ErrEndpointNotFound) {
		return OAuthImportResult{}, err
	}
	if existingID != "" {
		return s.updateOAuthEndpoint(ctx, existingID, credential, account, now)
	}
	return s.createOAuthEndpoint(ctx, providerID, credential, account, now)
}

// sealOAuthCredential seals both tokens, so the aggregate only ever holds
// ciphertext (SPEC-API-001 §6). The hint is derived from the plaintext here, at the
// only moment it exists, rather than read back from anything stored.
func (s *EndpointService) sealOAuthCredential(account OAuthAccountInput, now time.Time) (*domain.OAuthCredential, error) {
	accessSealed, err := s.sealer.Seal(strings.TrimSpace(account.AccessToken))
	if err != nil {
		return nil, domain.NewInternalError("the access token could not be stored")
	}
	refreshSealed := ""
	if refresh := strings.TrimSpace(account.RefreshToken); refresh != "" {
		refreshSealed, err = s.sealer.Seal(refresh)
		if err != nil {
			return nil, domain.NewInternalError("the refresh token could not be stored")
		}
	}
	refreshed := now
	return &domain.OAuthCredential{
		AccessTokenEncrypted:  accessSealed,
		RefreshTokenEncrypted: refreshSealed,
		ExpiresAt:             account.ExpiresAt,
		Scopes:                account.Scopes,
		AccountEmail:          account.Account.Email,
		AccountID:             account.Account.WorkspaceID,
		LastRefreshAt:         &refreshed,
	}, nil
}

// updateOAuthEndpoint writes a re-imported credential set onto the endpoint that
// already stands for the account.
func (s *EndpointService) updateOAuthEndpoint(ctx context.Context, id string, credential *domain.OAuthCredential, account OAuthAccountInput, now time.Time) (OAuthImportResult, error) {
	endpoint, err := s.store.GetByID(ctx, id)
	if err != nil {
		return OAuthImportResult{}, err
	}
	endpoint.SetOAuth(credential, now)
	endpoint.SetAccount(account.Account, now)
	if err := s.store.Update(ctx, endpoint); err != nil {
		return OAuthImportResult{}, err
	}
	return OAuthImportResult{
		Endpoint:  endpoint,
		Updated:   true,
		TokenHint: domain.MaskSecret(strings.TrimSpace(account.AccessToken)),
	}, nil
}

// createOAuthEndpoint stages a new oauth endpoint for an account never imported.
func (s *EndpointService) createOAuthEndpoint(ctx context.Context, providerID string, credential *domain.OAuthCredential, account OAuthAccountInput, now time.Time) (OAuthImportResult, error) {
	label := strings.TrimSpace(account.Label)
	if label == "" {
		label = defaultOAuthLabel(account.Account)
	}
	endpoint, err := domain.NewUpstreamEndpoint(domain.IDPrefixUpstreamEndpoint+domain.NewULID(now),
		providerID, label, domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		return OAuthImportResult{}, err
	}
	endpoint.SetOAuth(credential, now)
	endpoint.SetAccount(account.Account, now)
	if err := s.store.Create(ctx, endpoint); err != nil {
		return OAuthImportResult{}, err
	}
	return OAuthImportResult{
		Endpoint:  endpoint,
		TokenHint: domain.MaskSecret(strings.TrimSpace(account.AccessToken)),
	}, nil
}

// defaultOAuthLabel names an unlabelled account after the identity that
// distinguishes it, falling back to a fixed name when the import supplied no
// identity at all. Two accounts of one provider are told apart by exactly this.
func defaultOAuthLabel(account domain.EndpointAccount) string {
	switch {
	case account.Email != "":
		return account.Email
	case account.WorkspaceID != "":
		return "workspace-" + account.WorkspaceID
	case account.Name != "":
		return account.Name
	default:
		return "oauth-account"
	}
}

// rejectDuplicateLabels refuses a batch in which two rows claim one label under one
// provider, naming the earlier row so the client sees which pair collides.
func rejectDuplicateLabels(endpoints []domain.UpstreamEndpoint) error {
	seen := make(map[string]int, len(endpoints))
	for i, endpoint := range endpoints {
		key := endpoint.ProviderID() + "\x00" + endpoint.Label()
		if first, dup := seen[key]; dup {
			return rowError(i, domain.NewValidationError(
				"label is already used by row "+strconv.Itoa(first)))
		}
		seen[key] = i
	}
	return nil
}
