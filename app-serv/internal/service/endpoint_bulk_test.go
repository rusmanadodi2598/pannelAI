// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_bulk_test.go
// @for       Table-driven tests for the two batch routes and their all-or-nothing
//
//	rule (SPEC-API-001 §7.5, §8.1).
//
// @uses      context, errors, strconv, testing, time, internal/domain.
// @reason    §8.1's rule is the one that is worst to get wrong: a batch that wrote a
//
//	prefix of its rows leaves a half-imported account list, which is harder to reason
//	about than a refusal. Every negative case here asserts that NOTHING was written,
//	not merely that an error came back.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestEndpointService_BulkCreateIsAllOrNothing is the §8.1 requirement: a batch with
// one bad row writes no endpoint at all, and a valid batch writes every row.
func TestEndpointService_BulkCreateIsAllOrNothing(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		accounts   []BulkAccountInput
		wantCode   string
		wantStored int
	}{
		{
			name: "a valid batch stores every row",
			accounts: []BulkAccountInput{
				{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
				{Label: "three", Keys: []KeyInput{{Value: "sk-3"}}},
			},
			wantStored: 3,
		},
		{
			name: "a duplicate label in the last row writes nothing",
			accounts: []BulkAccountInput{
				{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
				{Label: "one", Keys: []KeyInput{{Value: "sk-3"}}},
			},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name: "a missing key on an api_key row writes nothing",
			accounts: []BulkAccountInput{
				{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two"},
			},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name: "a blank label in the first row writes nothing",
			accounts: []BulkAccountInput{
				{Label: "  ", Keys: []KeyInput{{Value: "sk-1"}}},
				{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
			},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name:     "an empty batch is refused",
			accounts: nil,
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name:     "an over-large batch is refused",
			accounts: makeAccounts(51),
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
		{
			name:     "an unknown provider is refused",
			provider: "nonexistent",
			accounts: []BulkAccountInput{{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}}},
			wantCode: "VALIDATION_ERROR", wantStored: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newEndpointSvc(t)
			provider := tc.provider
			if provider == "" {
				provider = "deepseek"
			}

			created, err := svc.BulkCreateAccounts(context.Background(), provider,
				domain.UpstreamAuthAPIKey, tc.accounts)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				if len(created) != 0 {
					t.Fatalf("a refused batch returned %d endpoints, want none", len(created))
				}
			} else if err != nil {
				t.Fatalf("BulkCreateAccounts() error = %v", err)
			}

			if len(store.byID) != tc.wantStored {
				t.Fatalf("stored endpoints = %d, want %d: a refused batch must write nothing",
					len(store.byID), tc.wantStored)
			}
		})
	}
}

// TestEndpointService_BulkCreateAttributesTheBadRow pins §8.1's per-row report for the
// account batch.
func TestEndpointService_BulkCreateAttributesTheBadRow(t *testing.T) {
	svc, _ := newEndpointSvc(t)

	_, err := svc.BulkCreateAccounts(context.Background(), "deepseek", domain.UpstreamAuthAPIKey,
		[]BulkAccountInput{
			{Label: "one", Keys: []KeyInput{{Value: "sk-1"}}},
			{Label: "two", Keys: []KeyInput{{Value: "sk-2"}}},
			{Label: "  ", Keys: []KeyInput{{Value: "sk-3"}}},
		})
	mustAppError(t, err, "VALIDATION_ERROR")

	var indexed BulkRowIndexer
	if !errors.As(err, &indexed) {
		t.Fatalf("error %v does not name the offending row", err)
	}
	index, ok := indexed.BulkRowIndex()
	if !ok || index != 2 {
		t.Fatalf("BulkRowIndex() = (%d, %v), want (2, true)", index, ok)
	}
}
