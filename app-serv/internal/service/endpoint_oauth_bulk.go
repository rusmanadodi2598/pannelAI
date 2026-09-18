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
//
// Every row is sealed and matched before the store is called, so a batch with one
// bad row writes nothing at all; the store then applies the whole set in a single
// transaction. That is the §8.1 rule this route shares with /endpoints/bulk, and
// it is why the loop below only builds values.
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

	now := s.clock()
	built := make([]domain.UpstreamEndpoint, 0, len(accounts))
	existing := make([]bool, 0, len(accounts))
	hints := make([]string, 0, len(accounts))
	for i, account := range accounts {
		endpoint, updated, hint, err := s.buildOAuthImport(ctx, providerID, account, now)
		if err != nil {
			return nil, rowError(i, err)
		}
		built = append(built, endpoint)
		existing = append(existing, updated)
		hints = append(hints, hint)
	}

	// Two rows claiming one label under one provider are refused here rather than
	// left to the unique index, so the refusal names the second row instead of
	// reporting a constraint without saying which row caused it.
	if err := rejectDuplicateLabels(built); err != nil {
		return nil, err
	}
	if err := s.store.ImportOAuthBatch(ctx, built, existing); err != nil {
		return nil, err
	}

	results := make([]OAuthImportResult, 0, len(built))
	for i, endpoint := range built {
		results = append(results, OAuthImportResult{
			Index:     i,
			Endpoint:  endpoint,
			Updated:   existing[i],
			TokenHint: hints[i],
		})
	}
	return results, nil
}

// buildOAuthImport seals one credential set and returns the endpoint that should
// stand for it: the account's existing endpoint with the new credential applied,
// or a fresh one. Nothing is written here, so a later row's refusal discards the
// work rather than leaving it half-applied.
func (s *EndpointService) buildOAuthImport(ctx context.Context, providerID string, account OAuthAccountInput, now time.Time) (domain.UpstreamEndpoint, bool, string, error) {
	accessToken := strings.TrimSpace(account.AccessToken)
	if accessToken == "" {
		return domain.UpstreamEndpoint{}, false, "", domain.NewValidationError("access_token is required")
	}
	credential, err := s.sealOAuthCredential(account, now)
	if err != nil {
		return domain.UpstreamEndpoint{}, false, "", err
	}
	hint := domain.MaskSecret(accessToken)

	existingID, err := s.store.FindOAuthEndpoint(ctx, providerID, account.Account.Email, account.Account.WorkspaceID)
	if err != nil && !errors.Is(err, domain.ErrEndpointNotFound) {
		return domain.UpstreamEndpoint{}, false, "", err
	}
	if existingID != "" {
		endpoint, err := s.store.GetByID(ctx, existingID)
		if err != nil {
			return domain.UpstreamEndpoint{}, false, "", err
		}
		endpoint.SetOAuth(credential, now)
		endpoint.SetAccount(account.Account, now)
		return endpoint, true, hint, nil
	}

	endpoint, err := buildOAuthEndpoint(providerID, credential, account, now)
	if err != nil {
		return domain.UpstreamEndpoint{}, false, "", err
	}
	return endpoint, false, hint, nil
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

// buildOAuthEndpoint stages a new oauth endpoint for an account never imported.
// It writes nothing: the batch transaction is the only writer on this path.
func buildOAuthEndpoint(providerID string, credential *domain.OAuthCredential, account OAuthAccountInput, now time.Time) (domain.UpstreamEndpoint, error) {
	label := strings.TrimSpace(account.Label)
	if label == "" {
		label = defaultOAuthLabel(account.Account)
	}
	endpoint, err := domain.NewUpstreamEndpoint(domain.IDPrefixUpstreamEndpoint+domain.NewULID(now),
		providerID, label, domain.UpstreamAuthOAuth, 1, now)
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	endpoint.SetOAuth(credential, now)
	endpoint.SetAccount(account.Account, now)
	return endpoint, nil
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
