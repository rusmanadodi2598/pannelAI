// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_invalidate_test.go
// @for       That a stored-node mutation invalidates a caching provider index so the data plane sees the node on its next lookup.
// @uses      context, testing, time, internal/domain, internal/registry.
// @reason    R06 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: a cached overlay without invalidation would serve a provider set that lags the operator's own writes, which reintroduces the bug the runtime index exists to prevent. The invalidation is a capability of the index, so the service calls it when the index opts in, and this test pins that Create and Delete each do.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// invalidatingIndex is a ProviderIndex that records how often the service told
// it the stored-node overlay changed.
type invalidatingIndex struct {
	readinessProviderIndex
	invalidations int
}

func (i *invalidatingIndex) InvalidateNodeOverlay() { i.invalidations++ }

// TestNodeService_MutationsInvalidateTheOverlayCache pins that Create and
// Delete each notify a caching index exactly once, so a node an operator just
// created or removed is visible on the next lookup, not after a TTL.
func TestNodeService_MutationsInvalidateTheOverlayCache(t *testing.T) {
	store := newReadinessNodeStore()
	index := &invalidatingIndex{readinessProviderIndex: readinessProviderIndex{
		entries: []registry.Provider{{ID: "openai"}},
	}}
	svc, err := NewNodeService(NodeServiceDeps{
		Store: store, Index: index, Counts: readinessEndpointCounts{},
	})
	if err != nil {
		t.Fatalf("NewNodeService() error = %v", err)
	}
	svc.clock = func() time.Time { return time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC) }

	created, err := svc.Create(context.Background(), CreateNodeInput{
		Name: "one", Prefix: "node-one", Type: domain.NodeOpenAICompatible,
		APIType: domain.NodeAPIChat, BaseURL: "https://one.example",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if index.invalidations != 1 {
		t.Fatalf("Create invalidated the overlay %d times, want once", index.invalidations)
	}

	if err := svc.Delete(context.Background(), created.ID()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if index.invalidations != 2 {
		t.Fatalf("Delete invalidated the overlay %d times in total, want twice", index.invalidations)
	}
}
