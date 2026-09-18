// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node.go
// @for       The custom provider node lifecycle: create, list, inspect, patch,
//
//	delete, and connectivity test (SPEC-API-001 §7.4).
//
// @uses      internal/domain, internal/repository, context, strings, time.
// @reason    §7.4 lets an operator define their own OpenAI-compatible or
//
//	Anthropic-compatible base URL, and a node's prefix becomes a
//	model-string namespace. That makes two rules the service owns: the
//	prefix must not collide with a registry identifier or alias, and a
//	delete must refuse while an endpoint still references the node —
//	both of which need the registry and the endpoints table, not just the
//	node row (AGENTS.md §1.5 keeps that orchestration here).
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

// NodeService implements SPEC-API-001 §7.4.
type NodeService struct {
	store  repository.NodeRepository
	index  ProviderIndex
	counts EndpointCounter
	prober NodeProber
	clock  func() time.Time
}

// EndpointCounter reports how many endpoints reference a provider id, which is what
// makes a node delete refuse while one still does (domain.ErrNodeInUse). It is
// satisfied by the concrete endpoint repository and by the node repository, both of
// which already read that count.
type EndpointCounter interface {
	CountEndpoints(ctx context.Context, providerID string) (int64, error)
}

// NodeServiceDeps holds the collaborators the service needs. Prober may be nil: a
// deployment without one still serves node CRUD.
type NodeServiceDeps struct {
	Store  repository.NodeRepository
	Index  ProviderIndex
	Counts EndpointCounter
	Prober NodeProber
}

// NewNodeService validates deps and returns a ready service.
func NewNodeService(deps NodeServiceDeps) (*NodeService, error) {
	if deps.Store == nil {
		return nil, domain.NewValidationError("provider node store is required")
	}
	if deps.Index == nil {
		return nil, domain.NewValidationError("provider index is required")
	}
	if deps.Counts == nil {
		return nil, domain.NewValidationError("endpoint counter is required")
	}
	return &NodeService{
		store:  deps.Store,
		index:  deps.Index,
		counts: deps.Counts,
		prober: deps.Prober,
		clock:  time.Now,
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
	return node, nil
}

// Delete removes a node, refusing while an endpoint still references it
// (domain.ErrNodeInUse). The reference is by provider id string rather than by
// foreign key — a built-in provider has no node row at all — so the check has to
// run here rather than be declared in the schema.
func (s *NodeService) Delete(ctx context.Context, id string) error {
	node, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	count, err := s.counts.CountEndpoints(ctx, node.ID())
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrNodeInUse
	}
	return s.store.Delete(ctx, id)
}
