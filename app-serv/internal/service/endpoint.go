// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint.go
// @for       The upstream endpoint lifecycle: create, list, inspect, patch,
//
//	delete, and connectivity test (SPEC-API-001 §7.5).
//
// @uses      internal/domain, internal/repository, context, strings, time.
// @reason    §7.5 makes the endpoint the account an operator configures, and
//
//	AGENTS.md §1.5 puts orchestration here with no net/http import: this
//	layer validates a provider_id against the registry, seals every
//	credential before the aggregate sees it, renumbers siblings when a
//	priority changes, and records a probe's outcome through the
//	aggregate's own methods rather than by writing fields.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// maxKeysPerEndpoint bounds the keys one account may hold, so one request cannot
// turn an endpoint into an unbounded set of credentials. It matches the cap the
// schema enforces on a keys array.
const maxKeysPerEndpoint = 100

// EndpointService implements SPEC-API-001 §7.5.
type EndpointService struct {
	store   EndpointStore
	index   ProviderIndex
	sealer  CredentialSealer
	prober  EndpointProber
	proxies ProxyPoolFinder
	clock   func() time.Time
}

// EndpointServiceDeps holds the collaborators the service needs. Prober may be
// nil: a deployment without one still serves CRUD, and the test route then says
// testing is unavailable instead of panicking.
type EndpointServiceDeps struct {
	Store  EndpointStore
	Index  ProviderIndex
	Sealer CredentialSealer
	Prober EndpointProber
	// Proxies is optional: a deployment that wires none refuses a
	// `proxy_pool_id` rather than storing a name nothing can resolve.
	Proxies ProxyPoolFinder
}

// NewEndpointService validates deps and returns a ready service.
func NewEndpointService(deps EndpointServiceDeps) (*EndpointService, error) {
	if deps.Store == nil {
		return nil, domain.NewValidationError("endpoint store is required")
	}
	if deps.Index == nil {
		return nil, domain.NewValidationError("provider index is required")
	}
	if deps.Sealer == nil {
		return nil, domain.NewValidationError("credential sealer is required")
	}
	return &EndpointService{
		store:   deps.Store,
		index:   deps.Index,
		sealer:  deps.Sealer,
		prober:  deps.Prober,
		proxies: deps.Proxies,
		clock:   time.Now,
	}, nil
}

// CreateInput is one account to create, in domain-neutral terms. It is a struct
// rather than a parameter list because the same fields serve the single route and
// each element of the bulk route (§8.1: one shape, two entry points).
type CreateInput struct {
	ProviderID string
	Label      string
	AuthType   domain.UpstreamAuthType
	Priority   int
	Keys       []KeyInput
}

// KeyInput is one credential to store. Value is plaintext on entry and is sealed
// before it reaches the aggregate.
type KeyInput struct {
	Label    string
	Value    string
	Priority int
}

// Create validates and persists one account. An unknown provider_id is a
// VALIDATION_ERROR, and an api_key endpoint with no key is refused before any
// write: §7.5 requires at least one key for that auth type, and storing an account
// that can never route is worse than refusing the request.
func (s *EndpointService) Create(ctx context.Context, in CreateInput) (domain.UpstreamEndpoint, error) {
	endpoint, err := s.buildEndpoint(in, s.clock())
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	if err := s.store.Create(ctx, endpoint); err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	return endpoint, nil
}

// List returns one page of endpoints, narrowed by the caller's filter.
func (s *EndpointService) List(ctx context.Context, providerID, status string, page, perPage int) ([]domain.UpstreamEndpoint, int64, error) {
	filter := repository.EndpointFilter{
		ProviderID: strings.TrimSpace(providerID),
		Status:     strings.TrimSpace(status),
	}
	if filter.Status != "" {
		if _, err := domain.ParseUpstreamEndpointStatus(filter.Status); err != nil {
			return nil, 0, err
		}
	}
	return s.store.List(ctx, filter, repository.PageQuery{Page: page, PerPage: perPage})
}

// Get returns one endpoint with its keys.
func (s *EndpointService) Get(ctx context.Context, id string) (domain.UpstreamEndpoint, error) {
	if strings.TrimSpace(id) == "" {
		return domain.UpstreamEndpoint{}, domain.NewValidationError("id is required")
	}
	return s.store.GetByID(ctx, id)
}

// UpdatePatch is a partial change to an endpoint's own fields. A nil field means
// "leave unchanged"; keys are absent from this type entirely, because a key change
// is a different concern with its own methods.
type UpdatePatch struct {
	Label    *string
	Priority *int
	Status   *string

	// The connection-parity fields (draft 017 §4.1b).
	DefaultModel   *string
	GlobalPriority *int
	ProxyPoolID    *string
}

// Update applies a PATCH. A priority change renumbers the provider's other
// endpoints through Reorder, because two endpoints claiming one slot would make
// the router's order depend on which row PostgreSQL happened to return first
// (§7.5).
func (s *EndpointService) Update(ctx context.Context, id string, patch UpdatePatch) (domain.UpstreamEndpoint, error) {
	endpoint, err := s.store.GetByID(ctx, id)
	if err != nil {
		return domain.UpstreamEndpoint{}, err
	}

	label := endpoint.Label()
	if patch.Label != nil {
		label = *patch.Label
	}
	priority := endpoint.Priority()
	if patch.Priority != nil {
		priority = *patch.Priority
	}
	status := ""
	if patch.Status != nil {
		status = *patch.Status
	}
	reordered := patch.Priority != nil && *patch.Priority != endpoint.Priority()

	if err := endpoint.Update(label, priority, status, s.clock()); err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	if err := s.applyRouting(ctx, &endpoint, patch); err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	if err := s.store.Update(ctx, endpoint); err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	if !reordered {
		return endpoint, nil
	}

	if err := s.reorderSiblings(ctx, endpoint); err != nil {
		return domain.UpstreamEndpoint{}, err
	}
	// The renumber rewrote this endpoint's priority too, so the value is reloaded
	// rather than reported from the pre-reorder state: a client that trusted the
	// response would otherwise see the number it asked for while the list shows a
	// different one.
	return s.store.GetByID(ctx, id)
}

// Delete removes one endpoint and, by cascade, its keys.
func (s *EndpointService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return domain.NewValidationError("id is required")
	}
	return s.store.Delete(ctx, id)
}
