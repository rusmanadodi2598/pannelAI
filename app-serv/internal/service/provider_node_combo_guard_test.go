// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_combo_guard_test.go
// @for       The node-delete guard draft 028 F3 adds: a delete refuses while a
//
//	stored combo still lists the node as a member, in either spelling the
//	router accepts.
//
// @uses      internal/domain, internal/registry, context, strings, testing, time.
// @reason    A combo member that outlives its provider is attempted on every
//
//	request and answers with a refusal about a model the client never
//	named, which is exactly the defect draft 028 measured. The guard is
//	the write-time half of the fix, so it is pinned here rather than only
//	in the relay tests that pin the read-time half.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestNodeService_DeleteRefusesWhileAComboReferencesTheNode pins both spellings
// the router accepts (the node's id and its prefix) plus the two ways the guard
// must not over-reach: an unrelated provider, and a prefix that merely starts
// with the node's own.
func TestNodeService_DeleteRefusesWhileAComboReferencesTheNode(t *testing.T) {
	store := newReadinessNodeStore()
	combos := newStubComboRepo()
	svc, err := NewNodeService(NodeServiceDeps{
		Store: store, Index: readinessProviderIndex{entries: []registry.Provider{{ID: "openai"}}},
		Counts: readinessEndpointCounts{}, Combos: combos,
	})
	if err != nil {
		t.Fatalf("NewNodeService() error = %v", err)
	}
	node, err := svc.Create(context.Background(), CreateNodeInput{
		Name: "mine", Prefix: "mine", Type: domain.NodeOpenAICompatible, APIType: domain.NodeAPIChat,
		BaseURL: "https://mine.example",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// An unrelated provider, and a prefix that only starts with the node's own,
	// must not block the delete; they stay stored while the blocking ones are
	// checked, so the final delete proves the guard does not over-reach.
	seedGuardCombo(t, combos, "unrelated", "someoneelse/gpt-4o")
	seedGuardCombo(t, combos, "substring", "minetoo/gpt-4o")

	// The id spelling blocks it, and the refusal names the combo.
	blocking := seedGuardCombo(t, combos, "by-id", node.ID()+"/gpt-4o")
	assertComboBlocksDelete(t, svc, store, node.ID(), blocking.Name())
	dropGuardCombo(combos, blocking)

	// So does the prefix spelling, which the router accepts just as it accepts
	// the id (combo_order.go canonicalizes a member through the same lookup).
	blocking = seedGuardCombo(t, combos, "by-prefix", node.Prefix()+"/gpt-4o")
	assertComboBlocksDelete(t, svc, store, node.ID(), blocking.Name())
	dropGuardCombo(combos, blocking)

	if err := svc.Delete(context.Background(), node.ID()); err != nil {
		t.Fatalf("delete after the blocking combos are gone: %v", err)
	}
}

// assertComboBlocksDelete asserts the delete is refused with the combo named,
// and that the refused delete left the node in place.
func assertComboBlocksDelete(t *testing.T, svc *NodeService, store *readinessNodeStore, nodeID, comboName string) {
	t.Helper()
	err := svc.Delete(context.Background(), nodeID)
	mustAppError(t, err, "CONFLICT")
	if !strings.Contains(err.Error(), comboName) {
		t.Fatalf("refusal = %v, want it to name combo %q", err, comboName)
	}
	if _, ok := store.nodes[nodeID]; !ok {
		t.Fatal("the refused delete removed the node")
	}
}

// seedGuardCombo stores one single-member fallback combo naming the ref.
func seedGuardCombo(t *testing.T, combos *stubComboRepo, name, ref string) domain.Combo {
	t.Helper()
	combo, err := domain.NewCombo("cmb_"+name, name, domain.ComboFallback, 0, "",
		[]domain.ComboModel{comboRef(t, ref, 1)}, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewCombo(%q) error = %v", name, err)
	}
	if err := combos.Create(context.Background(), combo); err != nil {
		t.Fatalf("seeding combo %q: %v", name, err)
	}
	return combo
}

// dropGuardCombo removes a seeded combo from the stub, which is what an operator
// cleaning up before the delete does.
func dropGuardCombo(combos *stubComboRepo, combo domain.Combo) {
	delete(combos.byID, combo.ID())
	delete(combos.nameID, combo.Name())
}
