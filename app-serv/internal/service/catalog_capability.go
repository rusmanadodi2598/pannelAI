// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/catalog_capability.go
// @for       How one catalog row answers a capability question: what the
//
//	registry document declares, plus what the model-id resolver decides.
//
// @uses      internal/domain, internal/registry, strings.
// @reason    SPEC-API-001 §7.6 offers `?capability=vision|tools`, and draft 017
//
//	§4.4 measured both of them returning zero rows over the 507 models the
//	embedded registry declares. The cause was two sources of truth for one
//	word: the document carried media operations while the modality names
//	lived in a model-id pattern table no catalog read. Keeping the two
//	sources and their division of labour in one file is what makes the
//	split readable, and it keeps the merge orchestration in
//	model_catalog.go inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// registryCapabilityNames is the capability set a registry model is filtered by:
// what the document declares, plus what the resolver decides.
//
// Two sources, two jobs. The document's `capabilities:` list carries the media
// operations (`edit`, `mask`, `text2img`) — facts about a row an operator reads,
// which no model-id pattern can know. The resolver carries `vision` and `tools`
// — judgements about a model id, which the document does not enumerate. Draft
// 017 §4.4 measured what happens with only the first: both of the panel's two
// filters answered zero rows across 507 registered models.
//
// Only `vision` is added as a name. `tools` is the resolver's floor, so emitting
// it would add one string to nearly every row of the catalog to say nothing —
// the filter still answers it, because matchesCatalogFilter asks the resolver
// for the two modality names rather than reading this list for them.
func registryCapabilityNames(providerID string, model registry.Model) domain.ModelCapabilities {
	names := append([]string(nil), model.Capabilities...)
	if registry.Capabilities(providerID, model.ID).Vision {
		names = append(names, capabilityVision)
	}
	return domain.NewModelCapabilities(names...)
}

// capabilityVision is the resolver's name for reading images. It is spelled
// once because the catalog filter, the resolver, and the panel's vocabulary
// (`app-ui/src/lib/schemas/model.ts:57`) all use it.
const capabilityVision = "vision"

// matchesCatalogFilter applies the three documented query parameters.
func matchesCatalogFilter(model domain.CatalogModel, filter CatalogFilter) bool {
	if provider := strings.TrimSpace(filter.ProviderID); provider != "" && model.ProviderID() != provider {
		return false
	}
	if capability := strings.TrimSpace(filter.Capability); capability != "" && !modelHasCapability(model, capability) {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(model.ModelID()), query) ||
		strings.Contains(strings.ToLower(model.DisplayName()), query)
}

// modelHasCapability answers one capability question about one catalog row.
//
// The two modality names are resolved rather than read, because they are
// judgements about a model id: a custom row has no registry entry behind it, so
// the resolver is asked about its model id directly. Everything else is a
// membership test against the row's own declared set, which is where the media
// operations live.
func modelHasCapability(model domain.CatalogModel, capability string) bool {
	if model.Capabilities().Has(capability) {
		return true
	}
	resolved := registry.Capabilities(model.ProviderID(), model.ModelID())
	if !resolved.Has(capability) {
		return false
	}
	// A resolved answer must not resurrect a capability the operator removed: a
	// custom row that declares its own set is the operator's statement, and the
	// resolver only adds what the document is silent about.
	return model.Source() == domain.CatalogSourceRegistry
}

// registryDisplayName falls back to the model id when the registry declares no
// display name, so the panel never renders an empty cell.
func registryDisplayName(model registry.Model) string {
	if strings.TrimSpace(model.Name) == "" {
		return model.ID
	}
	return model.Name
}
