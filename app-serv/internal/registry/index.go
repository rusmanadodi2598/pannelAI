// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/index.go
// @for       The read-only lookups the rest of the system performs on the loaded
//
//	registry: by identifier, by model, and by category.
//
// @uses      internal/registry, sort, strings.
// @reason    SPEC-API-001 §7.4 serves the registry over HTTP and the data plane
//
//	resolves every model string through it, so the lookups live in one
//	place with the ordering rules written down; nothing here mutates
//	the index, which is why the accessors return copies.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import "sort"

// Revision reports the reference revision the document was generated from.
func (i *Index) Revision() string { return i.revision }

// Count reports how many providers the registry holds.
func (i *Index) Count() int { return len(i.providers) }

// All returns every provider. The slice is a copy, so a caller cannot reorder
// the index by sorting or truncating what it receives.
func (i *Index) All() []Provider {
	out := make([]Provider, len(i.providers))
	copy(out, i.providers)
	return out
}

// Provider resolves any identifier (id, alias, or extra alias) to its entry.
func (i *Index) Provider(name string) (Provider, bool) {
	id, ok := i.byName[name]
	if !ok {
		return Provider{}, false
	}
	return i.providers[i.byID[id]], true
}

// Model resolves a model inside a provider, by either identifier.
func (i *Index) Model(providerName, modelID string) (Model, bool) {
	provider, ok := i.Provider(providerName)
	if !ok {
		return Model{}, false
	}
	for _, model := range provider.Models {
		if model.ID == modelID {
			return model, true
		}
	}
	return Model{}, false
}

// ByCategory returns the providers in one category, ordered by priority then id
// so the list endpoint pages it deterministically (SPEC-API-001 §7.4).
func (i *Index) ByCategory(category string) []Provider {
	out := make([]Provider, 0, len(i.providers))
	for _, provider := range i.providers {
		if provider.Category == category {
			out = append(out, provider)
		}
	}
	sortProviders(out)
	return out
}

// Categories returns every distinct category the document uses, sorted. The
// reference treats category as an open string, so this is measured rather than
// hardcoded and the provider filter is validated against it.
func (i *Index) Categories() []string {
	seen := make(map[string]struct{}, 5)
	for _, provider := range i.providers {
		seen[provider.Category] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for category := range seen {
		out = append(out, category)
	}
	sort.Strings(out)
	return out
}

// sortProviders orders by priority then id. Priority is the field the router
// reads, and the id tiebreak keeps two providers of equal priority in a stable
// order rather than in whatever order the map iteration produced.
func sortProviders(providers []Provider) {
	sort.SliceStable(providers, func(a, b int) bool {
		if providers[a].Priority != providers[b].Priority {
			return providers[a].Priority < providers[b].Priority
		}
		return providers[a].ID < providers[b].ID
	})
}

// Routability of a provider's chat path, as a closed set. The distinction
// matters because a provider can be listed and configured yet still not
// answerable: P1 translates the OpenAI, Anthropic, and OpenAI Responses wire
// formats, and a provider on any other protocol needs its own connector before
// it can serve a request (SPEC-API-001 §10, §7.15).
const (
	// Routable means the provider's declared format is one the gateway
	// translates natively.
	Routable = "native"

	// RoutableNeedsConnector means the provider speaks a protocol the gateway
	// does not translate, so it is answerable only once a connector for it is
	// registered. The panel surfaces this instead of offering an endpoint that
	// would always fail.
	RoutableNeedsConnector = "connector"
)

// nativeFormats are the wire formats the gateway translates itself.
var nativeFormats = map[string]struct{}{
	DefaultFormat:         {},
	"claude":              {},
	FormatOpenAIResponses: {},
}

// ChatRoutability reports whether this provider's chat path can be served
// today, and why not when it cannot.
//
// Reporting it here rather than at call time is the point: a provider the
// operator can add but that can never answer is a silent trap, and the answer
// belongs beside the data that decides it.
func (p Provider) ChatRoutability() string {
	if _, ok := nativeFormats[normalizeFormat(p.Transport.Format)]; ok {
		return Routable
	}
	return RoutableNeedsConnector
}

// IsChatRoutable reports whether the provider's chat path is served natively.
func (p Provider) IsChatRoutable() bool { return p.ChatRoutability() == Routable }
