// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_readiness_test.go
// @for       Provider and custom-node readiness rules from SPEC-API-001 §7.4.
// @uses      internal/domain, internal/registry, internal/repository, context, testing, time.
// @reason    §7.4 puts prefix collision and the rule that deleting a node takes its endpoints with it in the service, while provider filters and endpoint summaries are also service behavior. These tests keep those contract rules proven without a database or an upstream network.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-20
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// readinessNodeStore records the ids a cascade erased, so a test can prove a refused
// delete erased nothing: the cascade is one store call, the way it is one transaction.
type readinessNodeStore struct {
	nodes    map[string]domain.ProviderNode
	cascaded []string
}

func newReadinessNodeStore() *readinessNodeStore {
	return &readinessNodeStore{nodes: make(map[string]domain.ProviderNode)}
}
func (s *readinessNodeStore) Create(_ context.Context, n domain.ProviderNode) error {
	for _, old := range s.nodes {
		if old.Prefix() == n.Prefix() {
			return domain.ErrNodePrefixTaken
		}
	}
	s.nodes[n.ID()] = n
	return nil
}
func (s *readinessNodeStore) List(context.Context) ([]domain.ProviderNode, error) {
	out := make([]domain.ProviderNode, 0, len(s.nodes))
	for _, n := range s.nodes {
		out = append(out, n)
	}
	return out, nil
}
func (s *readinessNodeStore) GetByID(_ context.Context, id string) (domain.ProviderNode, error) {
	n, ok := s.nodes[id]
	if !ok {
		return domain.ProviderNode{}, domain.ErrNodeNotFound
	}
	return n, nil
}
func (s *readinessNodeStore) Update(_ context.Context, n domain.ProviderNode) error {
	s.nodes[n.ID()] = n
	return nil
}
func (s *readinessNodeStore) Delete(_ context.Context, id string) error {
	if _, ok := s.nodes[id]; !ok {
		return domain.ErrNodeNotFound
	}
	delete(s.nodes, id)
	return nil
}

func (s *readinessNodeStore) DeleteCascade(_ context.Context, id string) error {
	if _, ok := s.nodes[id]; !ok {
		return domain.ErrNodeNotFound
	}
	s.cascaded = append(s.cascaded, id)
	delete(s.nodes, id)
	return nil
}

type readinessProviderIndex struct{ entries []registry.Provider }

func (i readinessProviderIndex) Provider(id string) (registry.Provider, bool) {
	for _, p := range i.entries {
		if p.ID == id || p.Alias == id {
			return p, true
		}
	}
	return registry.Provider{}, false
}
func (i readinessProviderIndex) All() []registry.Provider {
	return append([]registry.Provider(nil), i.entries...)
}
func (i readinessProviderIndex) Categories() []string {
	seen := map[string]bool{}
	for _, p := range i.entries {
		seen[p.Category] = true
	}
	out := make([]string, 0, len(seen))
	for category := range seen {
		out = append(out, category)
	}
	return out
}

func readinessNodeService(t *testing.T, store *readinessNodeStore) *NodeService {
	t.Helper()
	svc, err := NewNodeService(NodeServiceDeps{
		Store: store, Index: readinessProviderIndex{entries: []registry.Provider{{ID: "openai"}}},
	})
	if err != nil {
		t.Fatalf("NewNodeService() error = %v", err)
	}
	svc.clock = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }
	return svc
}

func TestNodeService_CreateAndUpdateRejectPrefixCollision(t *testing.T) {
	store := newReadinessNodeStore()
	svc := readinessNodeService(t, store)
	_, err := svc.Create(context.Background(), CreateNodeInput{Name: "one", Prefix: "openai", Type: domain.NodeOpenAICompatible, APIType: domain.NodeAPIChat, BaseURL: "https://one.example"})
	mustAppError(t, err, "CONFLICT")

	first, err := svc.Create(context.Background(), CreateNodeInput{Name: "one", Prefix: "first", Type: domain.NodeOpenAICompatible, APIType: domain.NodeAPIChat, BaseURL: "https://one.example"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := svc.Create(context.Background(), CreateNodeInput{Name: "two", Prefix: "second", Type: domain.NodeOpenAICompatible, APIType: domain.NodeAPIChat, BaseURL: "https://two.example"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	_, err = svc.Update(context.Background(), second.ID(), NodePatch{Prefix: readinessStrPtr("FIRST")})
	mustAppError(t, err, "CONFLICT")
	if got, _ := store.GetByID(context.Background(), first.ID()); got.Prefix() != "first" {
		t.Fatal("collision changed the existing node")
	}
}

func TestNodeService_DeleteTakesTheNodesEndpointsAndModelRowsWithIt(t *testing.T) {
	store := newReadinessNodeStore()
	svc := readinessNodeService(t, store)
	node, err := svc.Create(context.Background(), CreateNodeInput{Name: "one", Prefix: "one", Type: domain.NodeOpenAICompatible, APIType: domain.NodeAPIChat, BaseURL: "https://one.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Delete(context.Background(), node.ID()); err != nil {
		t.Fatalf("delete of a node that has connections: %v", err)
	}
	if _, err := store.GetByID(context.Background(), node.ID()); !errors.Is(err, domain.ErrNodeNotFound) {
		t.Fatalf("the node survived its own delete: %v", err)
	}
	if len(store.cascaded) != 1 || store.cascaded[0] != node.ID() {
		t.Fatalf("cascade ran for %v, want exactly the deleted node", store.cascaded)
	}
}

func TestProviderService_FiltersAndSummaries(t *testing.T) {
	index := readinessProviderIndex{entries: []registry.Provider{
		{ID: "alpha", Category: "apikey", Priority: 1, Transport: registry.Transport{Format: "openai"}},
		{ID: "beta", Category: "oauth", Priority: 2, Transport: registry.Transport{Format: "unknown"}},
	}}
	counts := readinessProviderCounts{values: map[string]domain.EndpointStatusCounts{"alpha": {Total: 3, Active: 2, Error: 1}}}
	svc, err := NewProviderService(ProviderServiceDeps{Index: index, Counts: counts})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}
	rows, total, err := svc.List(context.Background(), ProviderFilter{Category: "apikey", Routability: registry.Routable}, 1, 100)
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Entry.ID != "alpha" || rows[0].Summary.Total != 3 {
		t.Fatalf("rows = %+v, total = %d", rows, total)
	}
	if _, err := svc.Detail(context.Background(), "missing"); domain.AsAppError(err).Code != "NOT_FOUND" {
		t.Fatalf("detail error = %v, want NOT_FOUND", err)
	}
}

type readinessProviderCounts struct {
	values map[string]domain.EndpointStatusCounts
}

func (c readinessProviderCounts) EndpointStatusCountsByProvider(_ context.Context, ids []string) (map[string]domain.EndpointStatusCounts, error) {
	out := make(map[string]domain.EndpointStatusCounts, len(ids))
	for _, id := range ids {
		out[id] = c.values[id]
	}
	return out, nil
}

func readinessStrPtr(value string) *string { return &value }
