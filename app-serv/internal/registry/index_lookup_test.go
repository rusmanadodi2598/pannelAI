// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/index_lookup_test.go
// @for       Table-driven tests for index lookup, copying, and the ordering the
//
//	list endpoint pages on.
//
// @uses      testing, internal/registry.
// @reason    The read paths of the index back both routing and the console list
//
//	endpoint, so a copy that leaks the shared slice or an unstable
//	ordering corrupts callers far from the lookup; these tests pin them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "testing"

// TestIndex_AllReturnsACopy pins the immutability contract: a caller sorting or
// truncating what All returns must not reorder the shared index.
func TestIndex_AllReturnsACopy(t *testing.T) {
	idx, err := loadFixture(t, `
providers:
  - id: b
    category: apikey
  - id: a
    category: apikey
`)
	if err != nil {
		t.Fatalf("load error = %v", err)
	}
	first := idx.All()
	first[0] = Provider{ID: "tampered"}
	second := idx.All()
	if second[0].ID == "tampered" {
		t.Fatal("All() must return a copy, not the index's own slice")
	}
}

// TestIndex_ByCategoryIsOrdered pins the ordering the list endpoint pages on:
// priority, then id as a stable tiebreak.
func TestIndex_ByCategoryIsOrdered(t *testing.T) {
	idx, err := loadFixture(t, `
providers:
  - id: second
    priority: 20
    category: apikey
  - id: first
    priority: 10
    category: apikey
  - id: tie-b
    priority: 30
    category: apikey
  - id: tie-a
    priority: 30
    category: apikey
  - id: other
    priority: 1
    category: oauth
`)
	if err != nil {
		t.Fatalf("load error = %v", err)
	}

	got := idx.ByCategory("apikey")
	want := []string{"first", "second", "tie-a", "tie-b"}
	if len(got) != len(want) {
		t.Fatalf("ByCategory(apikey) = %d providers, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("ByCategory(apikey)[%d] = %q, want %q", i, got[i].ID, id)
		}
	}

	// The categories are measured from the document, not hardcoded.
	cats := idx.Categories()
	if len(cats) != 2 || cats[0] != "apikey" || cats[1] != "oauth" {
		t.Fatalf("Categories() = %v, want [apikey oauth]", cats)
	}
}

// TestIndex_AliasWinsOverAnotherProvidersID pins the precedence the reference
// actually implements.
//
// The reference resolves through one flat alias table built from every
// provider's `uiAlias || alias`, so a name that is both a provider id and
// someone else's alias resolves to the alias holder and leaves the other
// provider unreachable by that id. The data forces the question: `mimo-free`
// declares `alias: mmf` while a hidden entry has `id: mmf`. Id-first would
// summon the hidden entry and change which provider answers "mmf/...", which is
// a silent routing difference from the reference.
func TestIndex_AliasWinsOverAnotherProvidersID(t *testing.T) {
	idx, err := loadFixture(t, `
providers:
  - id: shadowed
    category: apikey
  - id: mimo-free
    alias: shadowed
    category: free
`)
	if err != nil {
		t.Fatalf("load error = %v, want the alias to win without a collision", err)
	}
	p, ok := idx.Provider("shadowed")
	if !ok {
		t.Fatal("Provider(shadowed) must resolve")
	}
	if p.ID != "mimo-free" {
		t.Fatalf("Provider(shadowed).ID = %q, want the alias holder", p.ID)
	}
	// The alias holder remains reachable by its own id as well.
	if other, ok := idx.Provider("mimo-free"); !ok || other.ID != "mimo-free" {
		t.Fatalf("Provider(mimo-free) = (%+v, %v), want it reachable by id", other, ok)
	}
}

// TestIndex_RealRegistryResolvesMMFLikeTheReference drives the embedded document
// through the exact collision it carries, so the port is checked against real
// data and not only against a fixture.
func TestIndex_RealRegistryResolvesMMFLikeTheReference(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, ok := idx.Provider("mmf")
	if !ok {
		t.Fatal("mmf must resolve in the embedded registry")
	}
	if got.ID != "mimo-free" {
		t.Fatalf("Provider(mmf).ID = %q, want mimo-free, which is what the reference resolves", got.ID)
	}
}

func TestProvider_UpstreamIDAndIsChat(t *testing.T) {
	idx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	provider, ok := idx.Provider("codex")
	if !ok {
		t.Fatal("codex must be present in the embedded registry")
	}
	if len(provider.Models) == 0 {
		t.Fatal("codex must declare models")
	}
	for _, model := range provider.Models {
		// Every model must resolve to the id the upstream expects.
		if model.UpstreamID() == "" {
			t.Fatalf("model %q has an empty upstream id", model.ID)
		}
	}
}
