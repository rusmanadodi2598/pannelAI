// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_alias_guard_test.go
// @for       The node-delete guard for model aliases: a delete refuses while an alias targets the node, in either spelling the router accepts.
// @uses      internal/domain, internal/registry, context, sort, testing, time.
// @reason    A node delete takes its endpoints with it, so an alias left naming a provider with nothing under it answers failures for clients that call it by name. The alias is the client-facing name, which makes refusing and naming it the write-time half of that rule; the read-time half belongs to resolution.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-08
package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stubAliasLister answers a fixed alias-to-target set. The map is shared with the
// test so a case can add the blocking alias after the node exists, and the names
// are sorted so a refusal names the same alias on every run.
type stubAliasLister struct{ targets map[string]string }

func (s stubAliasLister) Aliases(context.Context) ([]domain.ModelAlias, error) {
	names := make([]string, 0, len(s.targets))
	for name := range s.targets {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]domain.ModelAlias, 0, len(names))
	for _, name := range names {
		out = append(out, domain.RehydrateModelAlias(name, s.targets[name]))
	}
	return out, nil
}

func TestNodeService_DeleteRefusesWhileAnAliasTargetsTheNode(t *testing.T) {
	store := newReadinessNodeStore()
	erasures := &readinessEndpointErasures{}
	aliases := stubAliasLister{targets: map[string]string{}}
	svc, err := NewNodeService(NodeServiceDeps{
		Store: store, Index: readinessProviderIndex{entries: []registry.Provider{{ID: "openai"}}},
		Endpoints: erasures, Aliases: aliases,
	})
	if err != nil {
		t.Fatalf("NewNodeService() error = %v", err)
	}
	svc.clock = func() time.Time { return time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC) }

	node, err := svc.Create(context.Background(), CreateNodeInput{
		Name: "mine", Prefix: "mine", Type: domain.NodeOpenAICompatible, APIType: domain.NodeAPIChat,
		BaseURL: "https://mine.example",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// An alias of another provider, and one whose target only starts with the
	// node's prefix, must not block the delete.
	aliases.targets["someone-else"] = "other-provider/gpt-4o"
	aliases.targets["substring"] = node.Prefix() + "too/gpt-4o"

	// The id spelling blocks it, and the refusal names the alias.
	aliases.targets["by-id"] = node.ID() + "/gpt-4o"
	assertAliasBlocksDelete(t, svc, store, erasures, node.ID(), "by-id")
	delete(aliases.targets, "by-id")

	// So does the prefix spelling, which the router resolves the same way it
	// resolves a combo member.
	aliases.targets["by-prefix"] = node.Prefix() + "/gpt-4o"
	assertAliasBlocksDelete(t, svc, store, erasures, node.ID(), "by-prefix")
	delete(aliases.targets, "by-prefix")

	if err := svc.Delete(context.Background(), node.ID()); err != nil {
		t.Fatalf("delete after the blocking aliases are gone: %v", err)
	}
	if len(erasures.providers) != 1 || erasures.providers[0] != node.ID() {
		t.Fatalf("connections erased for %v, want exactly the deleted node", erasures.providers)
	}
}

// assertAliasBlocksDelete asserts the delete is refused with the alias named, and
// that the refused delete removed neither the node nor any of its connections.
func assertAliasBlocksDelete(
	t *testing.T,
	svc *NodeService,
	store *readinessNodeStore,
	erasures *readinessEndpointErasures,
	nodeID, aliasName string,
) {
	t.Helper()

	err := svc.Delete(context.Background(), nodeID)
	mustAppError(t, err, "CONFLICT")
	if !strings.Contains(err.Error(), aliasName) {
		t.Fatalf("refusal = %v, want it to name alias %q", err, aliasName)
	}
	if _, ok := store.nodes[nodeID]; !ok {
		t.Fatal("the refused delete removed the node")
	}
	if len(erasures.providers) != 0 {
		t.Fatalf("the refused delete erased connections for %v", erasures.providers)
	}
}
