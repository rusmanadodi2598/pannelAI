// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/catalog_pair_map_test.go
// @for       That the custom-pair resolution in the models list reads one snapshot of the registry rather than a lookup per pair.
// @uses      context, testing, internal/domain.
// @reason    R06 of docs/DRAFT/042-CODE-REVIEW-FIXES.md: ModelList resolved every stored custom pair against the registry one pair at a time, so /models paid a registry lookup per operator row on top of the overlay build. The list already holds every entry from All(); this test pins that the pairs are answered from that snapshot, by counting the registry lookups a two-pair list takes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// countingIndex delegates to a real index and counts the Provider lookups,
// which is the cost the pair map exists to remove.
type countingIndex struct {
	inner         *registry.Index
	providerCalls int
}

func (c *countingIndex) Provider(name string) (registry.Provider, bool) {
	c.providerCalls++
	return c.inner.Provider(name)
}

func (c *countingIndex) Model(providerName, modelID string) (registry.Model, bool) {
	return c.inner.Model(providerName, modelID)
}

func (c *countingIndex) All() []registry.Provider { return c.inner.All() }

// TestModelList_ResolvesCustomPairsFromOneSnapshot pins that listing two custom
// rows costs zero per-pair registry lookups: both the node id spelling and the
// node prefix spelling are answered by the snapshot All() already produced.
func TestModelList_ResolvesCustomPairsFromOneSnapshot(t *testing.T) {
	lookup := fakeLookup{
		aliases:  map[string]string{},
		custom:   []domain.ModelRef{ref(t, nodeID, "kept"), ref(t, nodePfx, "deep-2")},
		active:   map[string]bool{nodeID: true},
		disabled: nil,
	}
	index := &countingIndex{inner: catalogIndex(t)}
	resolver, err := NewResolver(index, lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	list, err := resolver.ModelList(context.Background())
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	got := make([]string, 0, len(list.Data))
	for _, entry := range list.Data {
		got = append(got, entry.ID)
	}
	want := []string{"free/free-chat", nodePfx + "/deep-2", nodePfx + "/kept"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("listed = %v, want %v", got, want)
	}
	if index.providerCalls != 0 {
		t.Fatalf("Provider() called %d times for two custom pairs, want the snapshot map to answer every pair", index.providerCalls)
	}
}
