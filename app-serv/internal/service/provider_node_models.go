// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_models.go
// @for       The provider model read: which list a provider answers with, and where it came from.
// @uses      internal/domain, internal/registry, context.
// @reason    SPEC-API-001 §7.4 serves `GET /providers/{id}/models`, and a custom node's models are not in the embedded document (draft 017 §4.2). The registry provider answers from the document; a node's answer is read through NodeModelSource and falls back to whatever the registry already holds. Keeping that branch here, rather than in the handler or in the overlay, is what lets the detail route, the catalog, and the data plane share one answer, because they all read the same entry.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-23
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
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

// modelsFor answers one provider's model list. A registry provider is the
// document, so the origin is registry and no source is consulted. A custom node is
// asked, with three outcomes: the upstream answered, so its list with origin
// upstream; the upstream could not answer, so the entry's own list with origin
// registry and the warning the source reported; no source wired or the source
// errored, so the entry's own list with the fixed warning. No path returns an
// error for a node: a node whose upstream is down still routes as a passthrough
// provider, so failing the read would drop a working node over a list it needs.
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
		// An upstream that answered with nothing is not an answer: falling back to
		// what the overlay holds is strictly more useful than reporting an empty
		// list to the panel as the node's current state.
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

// ProbeTargets answers the model set a per-model probe can walk: the resolved
// list plus the rows the operator declared for the same provider. The halves are
// not interchangeable: a node's resolved list is its upstream's answer, empty
// when that upstream is unreachable, while declared rows are on the node's own
// screen and routable, and a registry provider's declared rows are its
// supplement. Walking only the resolved half would report nothing to test beside
// a table full of models. Where both name one model the resolved row wins, since
// it carries the kind that decides reachability; its name gap fills from declared.
func (s *ProviderService) ProbeTargets(
	ctx context.Context, providerID string,
) (ProviderModelList, []registry.Model, error) {
	list, err := s.Models(ctx, providerID)
	if err != nil {
		return ProviderModelList{}, nil, err
	}
	declared, err := s.declaredModels(ctx, list.Entry)
	if err != nil {
		return ProviderModelList{}, nil, err
	}
	return list, mergeProbeTargets(list.Entry.Models, declared), nil
}

// mergeProbeTargets joins the two halves in the provider's own order: resolved
// rows first, then the declared rows no resolved row names.
func mergeProbeTargets(resolved []registry.Model, declared []domain.CustomModel) []registry.Model {
	merged := make([]registry.Model, 0, len(resolved)+len(declared))
	seen := make(map[string]int, len(resolved)+len(declared))
	for _, model := range resolved {
		seen[model.ID] = len(merged)
		merged = append(merged, model)
	}
	for _, row := range declared {
		id := row.ModelID()
		if index, known := seen[id]; known {
			if merged[index].Name == "" {
				merged[index].Name = row.DisplayName()
			}
			continue
		}
		seen[id] = len(merged)
		merged = append(merged, registry.Model{ID: id, Name: row.DisplayName()})
	}
	return merged
}
