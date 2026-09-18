// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider.go
// @for       The provider catalog reads: list, detail, and model list, each
//
//	carrying the routability the panel needs before it offers an endpoint.
//
// @uses      internal/domain, internal/registry, context, sort, strings.
// @reason    SPEC-API-001 §7.4 serves the embedded registry over HTTP, and §8
//
//	makes "configured but never answerable" something the panel must be able to
//	see. Filtering, paging, and the store read each already belong to another
//	boundary, so this file is the orchestration between them and holds no SQL.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"context"
	"sort"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// EndpointCounterByProvider reports the stored endpoint state roll-up for a set
// of provider ids in one read, so a provider list page costs one extra query
// rather than one per row (AGENTS.md §1.7).
type EndpointCounterByProvider interface {
	EndpointStatusCountsByProvider(ctx context.Context, providerIDs []string) (map[string]domain.EndpointStatusCounts, error)
}

// ProviderService implements SPEC-API-001 §7.4.
type ProviderService struct {
	index  ProviderIndex
	counts EndpointCounterByProvider
}

// ProviderServiceDeps holds the collaborators the service needs. Counts is
// optional: without it a provider list still renders, with every roll-up
// reported as zero rather than the request failing.
type ProviderServiceDeps struct {
	Index  ProviderIndex
	Counts EndpointCounterByProvider
}

// NewProviderService validates deps and returns a ready service.
func NewProviderService(deps ProviderServiceDeps) (*ProviderService, error) {
	if deps.Index == nil {
		return nil, domain.NewValidationError("provider index is required")
	}
	return &ProviderService{index: deps.Index, counts: deps.Counts}, nil
}

// ProviderFilter narrows the provider list. A zero value matches everything,
// which is distinct from a filter matching nothing.
type ProviderFilter struct {
	Category    string
	Routability string
}

// ProviderRow is one provider with its stored state roll-up.
type ProviderRow struct {
	Entry   registry.Provider
	Summary domain.EndpointStatusCounts
}

// List returns one page of registry providers, filtered and ordered.
//
// Category is matched against the measured set rather than a hardcoded list: an
// unknown category is a VALIDATION_ERROR here, so a client asking for a category
// the registry does not use learns it is unknown instead of receiving an empty
// page it would read as "no such provider".
func (s *ProviderService) List(ctx context.Context, filter ProviderFilter, page, perPage int) ([]ProviderRow, int64, error) {
	providers, err := s.entries(filter, true)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(providers))

	start := (page - 1) * perPage
	if start > len(providers) {
		start = len(providers)
	}
	end := start + perPage
	if end > len(providers) {
		end = len(providers)
	}
	window := providers[start:end]

	summaries, err := s.summariesFor(ctx, window)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]ProviderRow, 0, len(window))
	for _, entry := range window {
		rows = append(rows, ProviderRow{Entry: entry, Summary: summaries[entry.ID]})
	}
	return rows, total, nil
}

// Detail returns one provider by id, alias, or node prefix.
func (s *ProviderService) Detail(ctx context.Context, providerID string) (ProviderRow, error) {
	entry, ok := s.index.Provider(strings.TrimSpace(providerID))
	if !ok {
		return ProviderRow{}, domain.NewNotFoundError("provider is not in the registry")
	}
	summaries, err := s.summariesFor(ctx, []registry.Provider{entry})
	if err != nil {
		return ProviderRow{}, err
	}
	return ProviderRow{Entry: entry, Summary: summaries[entry.ID]}, nil
}

// Models returns one provider's model list.
func (s *ProviderService) Models(ctx context.Context, providerID string) (registry.Provider, error) {
	entry, ok := s.index.Provider(strings.TrimSpace(providerID))
	if !ok {
		return registry.Provider{}, domain.NewNotFoundError("provider is not in the registry")
	}
	return entry, nil
}

// Categories returns the measured category set, which is what makes the list
// filter's rejection a statement of fact rather than of policy.
func (s *ProviderService) Categories() []string {
	return s.index.Categories()
}

// entries applies the filter and the ordering. Hidden entries are excluded:
// the reference marks them that way because their id is reachable only through
// another entry's alias, so listing them would offer a provider the operator
// cannot usefully configure on its own.
func (s *ProviderService) entries(filter ProviderFilter, excludeHidden bool) ([]registry.Provider, error) {
	providers := s.index.All()

	if filter.Category != "" {
		known := s.index.Categories()
		if !containsString(known, filter.Category) {
			return nil, domain.NewValidationError("unknown provider category " + filter.Category)
		}
	}

	out := make([]registry.Provider, 0, len(providers))
	for _, entry := range providers {
		if excludeHidden && entry.Hidden {
			continue
		}
		if filter.Category != "" && entry.Category != filter.Category {
			continue
		}
		if filter.Routability != "" && entry.ChatRoutability() != filter.Routability {
			continue
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Priority != out[b].Priority {
			return out[a].Priority < out[b].Priority
		}
		return out[a].ID < out[b].ID
	})
	return out, nil
}

// summariesFor reads the endpoint roll-up for a page's providers.
//
// The read is keyed by the page's ids, so its result set is bounded by the page
// size rather than by the registry (AGENTS.md §1.7). A store failure is
// returned rather than swallowed: reporting every provider as healthy because
// the count read failed is exactly the wrong direction for a panel whose job is
// to show which account is broken.
func (s *ProviderService) summariesFor(ctx context.Context, providers []registry.Provider) (map[string]domain.EndpointStatusCounts, error) {
	summaries := make(map[string]domain.EndpointStatusCounts, len(providers))
	if s.counts == nil || len(providers) == 0 {
		return summaries, nil
	}
	ids := make([]string, 0, len(providers))
	for _, entry := range providers {
		ids = append(ids, entry.ID)
	}
	measured, err := s.counts.EndpointStatusCountsByProvider(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, entry := range providers {
		summaries[entry.ID] = measured[entry.ID]
	}
	return summaries, nil
}

// containsString reports membership. The category set is single-digit, so a
// linear scan is cheaper than building a map on every request.
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
