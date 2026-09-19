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

// The resolution of registry defaults and stored overrides — viewsFor,
// viewFor, MediaBaseURL, overrideIndex, and the kind vocabulary mapping — lives
// in media_provider_resolve.go: every read asks that question, and this file is
// the use cases that ask it.

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
