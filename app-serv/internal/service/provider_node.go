// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node.go
// @for       The custom provider node lifecycle: create, list, inspect, patch, delete, and connectivity test (SPEC-API-001 §7.4).
// @uses      internal/domain, internal/repository, context, strings, time.
// @reason    §7.4 lets an operator define their own OpenAI-compatible or Anthropic-compatible base URL, and a node's prefix becomes a model-string namespace. That gives the service two rules the node row itself cannot hold: a prefix must not collide with a registry identifier or alias, and a delete has to take the node's endpoints and model rows with it while refusing a node a combo or an alias still names. Both need the registry, the endpoints table, the model catalog, the combo table and the alias set, not just the node row (AGENTS.md §1.5 keeps that orchestration here).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import (
	"context"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// NodeService implements SPEC-API-001 §7.4.
type NodeService struct {
	store     repository.NodeRepository
	index     ProviderIndex
	endpoints EndpointEraser
	models    ModelEraser
	combos    ComboLister
	aliases   AliasLister
	prober    NodeProber
	clock     func() time.Time
}

// EndpointEraser removes every endpoint that references a provider id, which is
// what lets a node delete take its connections with it. An endpoint is routed by
// the base URL and wire format stored on the node row, so a node gone leaves a
// connection nothing can answer with. The keys go through the cascade the schema
// declares on upstream_keys.endpoint_id.
type EndpointEraser interface {
	DeleteByProvider(ctx context.Context, providerID string) error
}

// ModelEraser removes the model rows stored under a provider id, custom and disabled
// alike. They key on the provider rather than reference it, so no database cascade
// clears them when the node goes, and without this every delete of a node that had
// declared models left rows nothing can read.
type ModelEraser interface {
	DeleteForProvider(ctx context.Context, providerID string) error
}

// NodeServiceDeps holds the collaborators the service needs. Prober may be nil: a
// deployment without one still serves node CRUD. Combos and Aliases may be nil: a
// deployment without those tables skips the reference checks they stand for.
type NodeServiceDeps struct {
	Store     repository.NodeRepository
	Index     ProviderIndex
	Endpoints EndpointEraser
	Models    ModelEraser
	Combos    ComboLister
	Aliases   AliasLister
	Prober    NodeProber
}

// NewNodeService validates deps and returns a ready service.
func NewNodeService(deps NodeServiceDeps) (*NodeService, error) {
	if deps.Store == nil {
		return nil, domain.NewValidationError("provider node store is required")
	}
	if deps.Index == nil {
		return nil, domain.NewValidationError("provider index is required")
	}
	if deps.Endpoints == nil {
		return nil, domain.NewValidationError("endpoint eraser is required")
	}
	if deps.Models == nil {
		return nil, domain.NewValidationError("model eraser is required")
	}
	return &NodeService{
		store:     deps.Store,
		index:     deps.Index,
		endpoints: deps.Endpoints,
		models:    deps.Models,
		combos:    deps.Combos,
		aliases:   deps.Aliases,
		prober:    deps.Prober,
		clock:     time.Now,
	}, nil
}

// CreateNodeInput is a validated node creation request (§7.4).
type CreateNodeInput struct {
	Name    string
	Prefix  string
	Type    domain.NodeType
	APIType string
	BaseURL string
}

// Create validates and persists a node.
//
// The prefix is checked against the embedded registry before the write: two
// providers answering to one model string is unresolvable, so a prefix that
// shadows an id or an alias is a CONFLICT rather than a row the loader later trips
// over (§7.4). The domain constructor enforces the prefix's own shape and the
// per-type api_type rule.
func (s *NodeService) Create(ctx context.Context, in CreateNodeInput) (domain.ProviderNode, error) {
	now := s.clock()
	node, err := domain.NewProviderNode("", in.Name, in.Prefix, in.Type, in.APIType, in.BaseURL, now)
	if err != nil {
		return domain.ProviderNode{}, err
	}
	if err := s.rejectPrefixCollision(ctx, node, ""); err != nil {
		return domain.ProviderNode{}, err
	}
	if err := s.store.Create(ctx, node); err != nil {
		return domain.ProviderNode{}, err
	}
	s.invalidateOverlay()
	return node, nil
}

// List returns every node, narrowed by an optional type filter.
func (s *NodeService) List(ctx context.Context, nodeType string) ([]domain.ProviderNode, error) {
	nodes, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(nodeType)
	if trimmed == "" {
		return nodes, nil
	}
	filter := domain.NodeType(trimmed)
	if filter != domain.NodeOpenAICompatible && filter != domain.NodeAnthropicCompatible {
		return nil, domain.NewValidationError("invalid type: " + trimmed)
	}
	filtered := make([]domain.ProviderNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Type() == filter {
			filtered = append(filtered, node)
		}
	}
	return filtered, nil
}

// Get returns one node.
func (s *NodeService) Get(ctx context.Context, id string) (domain.ProviderNode, error) {
	if strings.TrimSpace(id) == "" {
		return domain.ProviderNode{}, domain.NewValidationError("id is required")
	}
	return s.store.GetByID(ctx, id)
}

// NodePatch is a partial change to a node: the fields §7.4 PATCHes. Type and
// api_type are absent because they decide which wire format the node speaks and
// are therefore part of its identity rather than a mutable field.
type NodePatch struct {
	Name    *string
	Prefix  *string
	BaseURL *string
}

// Update applies a PATCH, refusing a prefix that would collide with the registry
// or with another node.
func (s *NodeService) Update(ctx context.Context, id string, patch NodePatch) (domain.ProviderNode, error) {
	node, err := s.store.GetByID(ctx, id)
	if err != nil {
		return domain.ProviderNode{}, err
	}
	now := s.clock()
	if patch.Name != nil {
		if err := node.Rename(*patch.Name, now); err != nil {
			return domain.ProviderNode{}, err
		}
	}
	if patch.Prefix != nil {
		candidate := domain.RehydrateProviderNode(node.ID(), node.Type(), node.Name(),
			*patch.Prefix, node.APIType(), node.BaseURL(), node.CreatedAt(), node.UpdatedAt())
		if err := s.rejectPrefixCollision(ctx, candidate, node.ID()); err != nil {
			return domain.ProviderNode{}, err
		}
		if err := node.Reprefix(*patch.Prefix, now); err != nil {
			return domain.ProviderNode{}, err
		}
	}
	if patch.BaseURL != nil {
		if err := node.Rebase(*patch.BaseURL, now); err != nil {
			return domain.ProviderNode{}, err
		}
	}
	if err := s.store.Update(ctx, node); err != nil {
		return domain.ProviderNode{}, err
	}
	s.invalidateOverlay()
	return node, nil
}

// Delete removes a node, the endpoints that reference it, and the model rows stored under
// its id. An endpoint is routed by the base URL on this row and `provider_id` is not
// writable on it, so its rows cannot outlive the node usefully and cannot be moved first;
// keys follow the schema's own cascade. A combo naming the node or an alias targeting its
// models is refused by name, before anything is erased. These references are id strings
// rather than foreign keys, so no schema constraint can carry any of it.
func (s *NodeService) Delete(ctx context.Context, id string) error {
	node, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.rejectComboReference(ctx, node.ID(), node.Prefix()); err != nil {
		return err
	}
	if err := s.rejectAliasReference(ctx, node.ID(), node.Prefix()); err != nil {
		return err
	}
	if err := s.endpoints.DeleteByProvider(ctx, node.ID()); err != nil {
		return err
	}
	if err := s.models.DeleteForProvider(ctx, node.ID()); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateOverlay()
	return nil
}

// overlayInvalidator is an optional ProviderIndex capability: a runtime index that
// caches the custom-node overlay clears it here, so a node write is visible on the
// next lookup instead of after the cache window.
type overlayInvalidator interface {
	InvalidateNodeOverlay()
}

// invalidateOverlay drops the cached overlay when the bound index keeps one. A
// static registry index has nothing to drop, so the assertion is the whole guard.
func (s *NodeService) invalidateOverlay() {
	if inv, ok := s.index.(overlayInvalidator); ok {
		inv.InvalidateNodeOverlay()
	}
}
