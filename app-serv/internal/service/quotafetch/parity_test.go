// Family/registry parity: the tripwire that makes a silent quota gap impossible.
//
// @file      internal/service/quotafetch/parity_test.go
// @for       Asserts the fetcher map and the registry's usage-flagged providers are the same set.
// @uses      internal/registry, internal/service/quotafetch, sort, testing.
// @reason    Fourteen providers were marked `usage: true` in the registry with no handler behind them, and a fifteenth fetcher was registered under an id the registry never carries, so it could only ever be reached from a test. Both directions of that mistake are one assertion each, and this screen's whole purpose is to show a provider's quota, so the set is checked rather than remembered.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"sort"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

func loadRegistry(t *testing.T) *registry.Index {
	t.Helper()

	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	return index
}

// TestFamilyFetchersExistInTheRegistry is the direction that catches dead code: a
// handler keyed by an id no endpoint can ever carry is unreachable, and it reads as
// support the product does not have.
func TestFamilyFetchersExistInTheRegistry(t *testing.T) {
	index := loadRegistry(t)

	for _, family := range sortedFamilies() {
		provider, known := index.Provider(family)
		if !known {
			t.Errorf("fetcher family %q is no registry provider id; nothing can route to it", family)
			continue
		}
		if !provider.Features.Usage {
			t.Errorf("fetcher family %q exists but the registry marks it usage:false", family)
		}
	}
}

// TestUsageProvidersHaveAFetcher is the direction that catches the silent gap: a provider
// the registry advertises as quota-capable, and that the panel therefore offers an
// "ask the provider" path for, but that answers the soft "not implemented" sentence.
func TestUsageProvidersHaveAFetcher(t *testing.T) {
	index := loadRegistry(t)

	missing := []string{}
	for _, provider := range index.All() {
		if !provider.Features.Usage {
			continue
		}
		if _, ok := familyFetchers[provider.ID]; !ok {
			missing = append(missing, provider.ID)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("registry providers marked usage:true with no fetcher, so their card can only read \"not implemented\": %v", missing)
	}
}

func sortedFamilies() []string {
	out := make([]string, 0, len(familyFetchers))
	for family := range familyFetchers {
		out = append(out, family)
	}
	sort.Strings(out)
	return out
}
