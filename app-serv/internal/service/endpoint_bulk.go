// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk.go
// @for       The all-or-nothing batch that creates several accounts at one
//
//	provider in a single call (SPEC-API-001 §7.5, §8.1).
//
// @uses      internal/domain, context, strings.
// @reason    §8.1 makes a batch all-or-nothing and requires every row's outcome to
//
//	be reported by index, so the whole batch is validated before anything
//	is written and a refusal names the offending row. Both routes share
//	that rule and the account-identity matching a re-import needs, which
//	is why they live together rather than beside the single-create path; a
//	duplicated rule per route is how one of them drifts.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// maxBatchRows bounds one batch, so a single request cannot ask the gateway to
// hold an unbounded set of aggregates in memory before its one transaction opens.
// It matches the request-level cap the schema enforces.
const maxBatchRows = 50

// BulkAccountInput is one account inside a batch: the single-create fields without
// the provider and auth type, which the batch states once (§8.1).
type BulkAccountInput struct {
	Label    string
	Priority int
	Keys     []KeyInput
}

// BulkCreateAccounts creates several endpoints under one provider in one
// transaction.
//
// Every row is built before the store is called, so a malformed row refuses the
// batch without a statement running; the store then writes the whole set in a
// single transaction, so a constraint the rows could only break together also
// leaves nothing behind (§8.1). The refusal carries the offending index.
func (s *EndpointService) BulkCreateAccounts(ctx context.Context, providerID string, authType domain.UpstreamAuthType, accounts []BulkAccountInput) ([]domain.UpstreamEndpoint, error) {
	if len(accounts) == 0 {
		return nil, domain.NewValidationError("endpoints is required")
	}
	if len(accounts) > maxBatchRows {
		return nil, domain.NewValidationError("a batch may hold at most 50 endpoints")
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
	for i, account := range accounts {
		endpoint, err := s.buildEndpoint(CreateInput{
			ProviderID: providerID,
			Label:      account.Label,
			AuthType:   authType,
			Priority:   account.Priority,
			Keys:       account.Keys,
		}, now)
		if err != nil {
			return nil, rowError(i, err)
		}
		built = append(built, endpoint)
	}

	// A duplicate label between two batch rows is refused here rather than left to
	// the unique index, so the refusal names the second row instead of reporting a
	// constraint without saying which row caused it.
	if err := rejectDuplicateLabels(built); err != nil {
		return nil, err
	}
	if err := s.store.CreateBatch(ctx, built); err != nil {
		return nil, err
	}
	return built, nil
}
