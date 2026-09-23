// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_models.go
// @for       The provider model read: which list a provider answers with, and
//
//	where it came from.
//
// @uses      internal/domain, internal/registry, context.
// @reason    SPEC-API-001 §7.4 serves `GET /providers/{id}/models`, and a custom
//
//	node's models are not in the embedded document (draft 017 §4.2). The
//	registry provider answers from the document; a node's answer is read
//	through NodeModelSource and falls back to whatever the registry already
//	holds. Keeping that branch here — rather than in the handler or in the
//	overlay — is what lets the detail route, the catalog, and the data plane
//	share one answer, because they all read the same entry.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ProviderModelList is one provider's model list plus the origin of that list.
type ProviderModelList struct {
	// Entry is the registry entry the caller asked for, with the models the
	// answer carries. For a node it is a copy carrying the resolved list.
	Entry registry.Provider
	// Source is ModelSourceUpstream or ModelSourceRegistry.
	Source string
	// Warning is empty when the list came from where Source says it did.
	Warning string
}

// modelsFor answers one provider's model list.
//
// A registry provider is the document, so the origin is registry and no source
// is consulted. A custom node is asked, and three outcomes are possible:
//
//   - the upstream answered → its list, origin upstream;
//   - the upstream could not answer → the entry's own list (what the overlay
//     holds), origin registry, with the warning the source reported;
//   - no source is wired, or the source errored → the entry's own list, origin
//     registry, with the fixed warning.
//
// No path returns an error for a node. A node whose upstream is down still
// routes (a passthrough provider accepts any model string), so failing the read
// would take a working node out of the panel over a list it does not need.
func (s *ProviderService) modelsFor(ctx context.Context, entry registry.Provider) ProviderModelList {
	answer := ProviderModelList{Entry: entry, Source: ModelSourceRegistry}
	if !entry.Custom || s.source == nil {
		return answer
	}

	list, err := s.source.ListNodeModels(ctx, entry.ID)
	if err != nil {
		answer.Warning = UpstreamUnavailableWarning
		return answer
	}
	if list.Source != ModelSourceUpstream || len(list.Models) == 0 {
		// An upstream that answered with nothing is not an answer: a node with
		// no list at all is the state draft 017 §4.2 measured, and falling back
		// is strictly more useful than reporting an empty list as current.
		answer.Warning = list.Warning
		if answer.Warning == "" {
			answer.Warning = UpstreamUnavailableWarning
		}
		return answer
	}

	resolved := entry
	resolved.Models = list.Models
	return ProviderModelList{Entry: resolved, Source: ModelSourceUpstream}
}
