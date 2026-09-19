// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_provider_resolve.go
// @for       Resolving registry defaults and stored overrides into effective
//
//	media values (SPEC-API-001 §7.10).
//
// @uses      internal/domain, internal/registry, context, strings.
// @reason    Every §7.10 read answers the same question — what does this
//
//	provider actually dial for this kind — so the resolution and the
//	kind-vocabulary mapping sit in one file rather than beside each
//	route that asks.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// MediaBaseURL reports the stored override's base URL for one provider and
// kind, or an empty string when none is stored (SPEC-API-001 §7.10). It is the
// read path the data plane uses, so a save takes effect on the next call
// rather than at the next boot.
func (s *MediaProviderService) MediaBaseURL(ctx context.Context, providerID string, kind domain.MediaKind) (string, error) {
	override, found, err := s.repo.Get(ctx, strings.TrimSpace(providerID), kind)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return strings.TrimSpace(override.BaseURL()), nil
}

// overrideIndex loads the stored overrides into a lookup keyed by provider and
// kind. The table holds one row per provider and kind an operator has touched,
// so it is read whole rather than per provider (AGENTS.md §1.7).
func (s *MediaProviderService) overrideIndex(ctx context.Context) (map[string]domain.MediaOverride, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	index := make(map[string]domain.MediaOverride, len(rows))
	for _, row := range rows {
		index[overrideKey(row.ProviderID(), row.Kind())] = row
	}
	return index, nil
}

// viewsFor builds one entry's views in the canonical kind order, so a list
// renders the same way twice — a Go map iterates in a random order, which is
// why the order comes from domain.MediaKinds() rather than from the map.
func viewsFor(entry registry.Provider, filter domain.MediaKind, overrides map[string]domain.MediaOverride) []MediaServiceView {
	views := make([]MediaServiceView, 0, len(entry.Media))
	for _, kind := range domain.MediaKinds() {
		if filter != "" && kind != filter {
			continue
		}
		media, ok := entry.Media.For(registryMediaKind(kind))
		if !ok {
			continue
		}
		views = append(views, viewFor(entry, kind, media, overrides[overrideKey(entry.ID, kind)]))
	}
	return views
}

// viewFor resolves one kind's effective values: the stored override wins over
// the registry, and each value reports where it came from so the panel can
// offer a reset only where one applies.
func viewFor(entry registry.Provider, kind domain.MediaKind, media registry.MediaConfig, override domain.MediaOverride) MediaServiceView {
	baseURL := strings.TrimSpace(override.BaseURL())
	baseURLSource := domain.MediaSourceOverride
	if baseURL == "" {
		baseURL = strings.TrimSpace(media.BaseURL)
		baseURLSource = domain.MediaSourceRegistry
	}
	defaultModel := strings.TrimSpace(override.DefaultModel())
	defaultModelSource := domain.MediaSourceOverride
	if defaultModel == "" {
		defaultModel = strings.TrimSpace(media.DefaultModel)
		defaultModelSource = domain.MediaSourceRegistry
	}
	return MediaServiceView{
		ProviderID:         entry.ID,
		ProviderName:       mediaProviderName(entry),
		Kind:               kind,
		BaseURL:            baseURL,
		BaseURLSource:      baseURLSource,
		DefaultModel:       defaultModel,
		DefaultModelSource: defaultModelSource,
		Models:             media.Models,
	}
}

// mediaProviderName prefers the registry's display name and falls back to the
// id, so a minimal entry still renders a label instead of a blank row.
func mediaProviderName(entry registry.Provider) string {
	if entry.Display.Name != "" {
		return entry.Display.Name
	}
	return entry.ID
}

// validateMediaModel refuses a default model the kind does not declare, the
// same rule §7.8 applies to the vision adapter: a model the registry does not
// list cannot be routed, so accepting it would store a value that always fails.
func validateMediaModel(media registry.MediaConfig, model string) error {
	model = strings.TrimSpace(model)
	if model == "" || len(media.Models) == 0 {
		return nil
	}
	for _, declared := range media.Models {
		if declared.ID == model {
			return nil
		}
	}
	return domain.NewValidationError("model " + model + " is not declared for this service")
}

// registryMediaKind maps the wire kind to the registry's own vocabulary. The
// registry spells the search kind `webSearch`; the wire spells it `search`.
func registryMediaKind(kind domain.MediaKind) registry.MediaKind {
	switch kind {
	case domain.MediaKindTTS:
		return registry.MediaTTS
	case domain.MediaKindSTT:
		return registry.MediaSTT
	case domain.MediaKindEmbedding:
		return registry.MediaEmbedding
	case domain.MediaKindImage:
		return registry.MediaImage
	case domain.MediaKindVideo:
		return registry.MediaVideo
	case domain.MediaKindSearch:
		return registry.MediaWebSearch
	default:
		return ""
	}
}

// overrideKey is the lookup key for a stored override.
func overrideKey(providerID string, kind domain.MediaKind) string {
	return providerID + "\x00" + string(kind)
}
