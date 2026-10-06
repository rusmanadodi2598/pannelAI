// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/catalog_list_read_test.go
// @for       The owner segment and the refusal behavior of GET /api/v1/models.
// @uses      context, testing, internal/domain
// @reason    The list is what a picker offers, so two rules beside the row set need pinning: a provider segment the operator never typed is not a usable answer, and a catalog read that failed must not come back as a short list.
//
//	They live apart from the row-set table because that table is already at the
//	size where a second concern in the same file stops reading as one question.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package dataplane

import (
	"context"
	"errors"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// errStub is the refusal a failing read returns, so a listing path that swallowed an
// error into an empty set would be caught rather than reported as "nothing is active".
var errStub = errors.New("catalog read refused")

// TestModelList_OwnedByNamesTheListedProvider pins the owner segment beside a node's
// custom row: the list is what a picker shows, and an owner that spells the node by its
// internal id is a fact the operator never typed and cannot route by.
func TestModelList_OwnedByNamesTheListedProvider(t *testing.T) {
	lookup := fakeLookup{
		aliases: map[string]string{},
		active:  map[string]bool{"lit": true, nodeID: true},
		custom:  []domain.ModelRef{ref(t, nodeID, "kept")},
	}
	resolver, err := NewResolver(catalogIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	list, err := resolver.ModelList(context.Background())
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	owners := make(map[string]string, len(list.Data))
	for _, entry := range list.Data {
		owners[entry.ID] = entry.OwnedBy
	}
	if owners["lit/lit-chat"] != "lit" {
		t.Errorf("owner of lit/lit-chat = %q, want lit", owners["lit/lit-chat"])
	}
	if owners[nodePfx+"/kept"] != nodePfx {
		t.Errorf("owner of %s/kept = %q, want %s", nodePfx, owners[nodePfx+"/kept"], nodePfx)
	}
}

// TestModelList_ReadFailureIsRefused pins that a failed catalog read answers an error
// rather than a short list: a list built from the half that did read would name models
// the caller cannot route and hide ones it can, with no sentence saying which happened.
func TestModelList_ReadFailureIsRefused(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "custom read fails"},
		{name: "active read fails"},
		{name: "disabled read fails"},
		{name: "combo read fails"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := failingLookup{base: fakeLookup{
				aliases: map[string]string{},
				active:  map[string]bool{"lit": true},
				custom:  []domain.ModelRef{ref(t, "lit", "extra")},
				combos:  map[string]domain.Combo{"c": comboRow("c", "lit/lit-chat")},
			}, refuse: tc.name}

			resolver, err := NewResolver(catalogIndex(t), lookup)
			if err != nil {
				t.Fatalf("building resolver: %v", err)
			}
			if _, err := resolver.ModelList(context.Background()); err == nil {
				t.Fatalf("ModelList with %q: want error, got nil", tc.name)
			}
		})
	}
}

// failingLookup refuses exactly one catalog read per case, so a stub that swallowed every
// error could not pass the test above.
type failingLookup struct {
	base   fakeLookup
	refuse string
}

func (f failingLookup) Combo(ctx context.Context, name string) (domain.Combo, bool, error) {
	return f.base.Combo(ctx, name)
}

func (f failingLookup) Alias(ctx context.Context, name string) (string, bool, error) {
	return f.base.Alias(ctx, name)
}

func (f failingLookup) Disabled(ctx context.Context, providerID, modelID string) (bool, error) {
	if f.refuse == "disabled read fails" {
		return false, errStub
	}
	return f.base.Disabled(ctx, providerID, modelID)
}

func (f failingLookup) DisabledPairs(ctx context.Context) ([]domain.ModelRef, error) {
	if f.refuse == "disabled read fails" {
		return nil, errStub
	}
	return f.base.DisabledPairs(ctx)
}

func (f failingLookup) ComboNames(ctx context.Context) ([]string, error) {
	if f.refuse == "combo read fails" {
		return nil, errStub
	}
	return f.base.ComboNames(ctx)
}

func (f failingLookup) CustomModels(ctx context.Context) ([]domain.ModelRef, error) {
	if f.refuse == "custom read fails" {
		return nil, errStub
	}
	return f.base.CustomModels(ctx)
}

func (f failingLookup) ActiveProviders(ctx context.Context, ids []string) (map[string]bool, error) {
	if f.refuse == "active read fails" {
		return nil, errStub
	}
	return f.base.ActiveProviders(ctx, ids)
}
