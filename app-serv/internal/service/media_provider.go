// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_provider.go
// @for       The media provider reads and the per-kind override save
//
//	(SPEC-API-001 §7.10).
//
// @uses      internal/domain, internal/registry, internal/repository, context,
//
//	sort, strings, time.
//
// @reason    §7.10 lists media-capable providers by kind and lets an operator
//
//	point one at their own host. The registry is the default and the
//	stored override wins, so this layer is where the two meet; it is
//	also where the wire kind (`search`) is mapped to the registry
//	kind (`webSearch`), so neither vocabulary leaks into the other.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// MediaServiceView is one provider's offering of one media kind, resolved: the
// effective base URL and default model are the override when one is stored and
// the registry's value otherwise, and each carries its own source because one
// value can be overridden while the other is not.
type MediaServiceView struct {
	ProviderID         string
	ProviderName       string
	Kind               domain.MediaKind
	BaseURL            string
	BaseURLSource      string
	DefaultModel       string
	DefaultModelSource string
	Models             []registry.MediaModel
	EndpointCount      int64
}

// MediaOverrideDraft is the patch input of §7.10.
type MediaOverrideDraft struct {
	Kind         domain.MediaKind
	BaseURL      string
	DefaultModel string
}

// MediaProviderServiceDeps holds the collaborators the service needs.
type MediaProviderServiceDeps struct {
	Index ProviderIndex
	Repo  repository.MediaOverrideRepository
	// Counts is optional: without it a list still renders, with every endpoint
	// count reported as zero rather than the request failing.
	Counts EndpointCounterByProvider
}

// MediaProviderService implements SPEC-API-001 §7.10.
type MediaProviderService struct {
	index  ProviderIndex
	repo   repository.MediaOverrideRepository
	counts EndpointCounterByProvider
	clock  func() time.Time
}

// NewMediaProviderService validates deps and returns a ready service.
func NewMediaProviderService(deps MediaProviderServiceDeps) (*MediaProviderService, error) {
	if deps.Index == nil {
		return nil, domain.NewValidationError("media provider service requires a provider index")
	}
	if deps.Repo == nil {
		return nil, domain.NewValidationError("media provider service requires an override repository")
	}
	return &MediaProviderService{
		index: deps.Index, repo: deps.Repo, counts: deps.Counts, clock: time.Now,
	}, nil
}

// List returns one view per provider and kind, filtered by kind when one is
// named. Hidden registry entries are skipped for the same reason §7.4 skips
// them: their id is reachable only through another entry's alias.
func (s *MediaProviderService) List(ctx context.Context, kind domain.MediaKind) ([]MediaServiceView, error) {
	entries := s.index.All()
	overrides, err := s.overrideIndex(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]MediaServiceView, 0, len(entries))
	for _, entry := range entries {
		if entry.Hidden {
			continue
		}
		views = append(views, viewsFor(entry, kind, overrides)...)
	}
	if err := s.attachCounts(ctx, views); err != nil {
		return nil, err
	}
	return views, nil
}

// Detail returns every media kind one provider offers, or a not-found error
// when the id is unknown or the provider offers none — the route is the media
// surface, so a provider with no media service is not on it.
func (s *MediaProviderService) Detail(ctx context.Context, providerID string) ([]MediaServiceView, error) {
	entry, ok := s.index.Provider(strings.TrimSpace(providerID))
	if !ok {
		return nil, domain.NewNotFoundError("provider is not in the registry")
	}
	overrides, err := s.overrideIndex(ctx)
	if err != nil {
		return nil, err
	}
	views := viewsFor(entry, "", overrides)
	if len(views) == 0 {
		return nil, domain.NewNotFoundError("provider " + entry.ID + " offers no media service")
	}
	if err := s.attachCounts(ctx, views); err != nil {
		return nil, err
	}
	return views, nil
}

// Patch saves one provider's per-kind override and returns the resolved view.
//
// The save is refused when it would leave the provider unusable — no base URL
// from either source — because §7.10 forbids a silent cloud fallback and a
// provider that cannot be dialed is exactly that (§7.10's "missing base_url ⇒
// VALIDATION_ERROR").
func (s *MediaProviderService) Patch(ctx context.Context, providerID string, draft MediaOverrideDraft) (MediaServiceView, error) {
	entry, ok := s.index.Provider(strings.TrimSpace(providerID))
	if !ok {
		return MediaServiceView{}, domain.NewNotFoundError("provider is not in the registry")
	}
	media, ok := entry.Media.For(registryMediaKind(draft.Kind))
	if !ok {
		return MediaServiceView{}, domain.NewValidationError(
			"provider " + entry.ID + " does not offer " + string(draft.Kind))
	}
	if err := validateMediaModel(media, draft.DefaultModel); err != nil {
		return MediaServiceView{}, err
	}
	override, err := domain.NewMediaOverride(entry.ID, draft.Kind, draft.BaseURL, draft.DefaultModel, s.clock().UTC())
	if err != nil {
		return MediaServiceView{}, err
	}
	if override.BaseURL() == "" && strings.TrimSpace(media.BaseURL) == "" {
		return MediaServiceView{}, domain.NewValidationError(
			"provider " + entry.ID + " has no " + string(draft.Kind) + " base_url; set one")
	}
	if err := s.repo.Upsert(ctx, override); err != nil {
		return MediaServiceView{}, err
	}
	view := viewFor(entry, draft.Kind, media, override)
	if err := s.attachCounts(ctx, []MediaServiceView{view}); err != nil {
		return MediaServiceView{}, err
	}
	return view, nil
}

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

// attachCounts fills each view's endpoint count in one read keyed by the
// distinct provider ids in the page.
func (s *MediaProviderService) attachCounts(ctx context.Context, views []MediaServiceView) error {
	if s.counts == nil || len(views) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(views))
	ids := make([]string, 0, len(views))
	for _, view := range views {
		if !seen[view.ProviderID] {
			seen[view.ProviderID] = true
			ids = append(ids, view.ProviderID)
		}
	}
	counts, err := s.counts.EndpointStatusCountsByProvider(ctx, ids)
	if err != nil {
		return err
	}
	for index := range views {
		views[index].EndpointCount = counts[views[index].ProviderID].Total
	}
	return nil
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
