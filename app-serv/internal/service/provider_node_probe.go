// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_node_probe.go
// @for       The provider node connectivity test and the node-to-registry mapping
//
//	a custom node needs to become routable.
//
// @uses      internal/domain, internal/registry, context, strings.
// @reason    SPEC-API-001 §7.4 requires a test that a node's base URL answers, and
//
//	a node has no test_status column — so unlike an endpoint's probe this
//	one returns a result rather than storing state. The registry mapping
//	lives beside it because both answer "is this node usable", and it
//	keeps registry.CustomNode construction out of the registry package,
//	which must not import the domain. It is separate from
//	provider_node.go because AGENTS.md §1.1 caps a file at 250 lines.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestNode probes a node's base URL and reports whether it answers.
//
// A node carries no credential of its own (§7.4), so the caller may supply one; an
// absent credential legitimately tests a node whose upstream needs none. The
// outcome is returned rather than stored: unlike an endpoint, a node has no
// test_status column, so a result is a response and not state.
func (s *NodeService) TestNode(ctx context.Context, id, credential string) (ProbeOutcome, error) {
	node, err := s.store.GetByID(ctx, id)
	if err != nil {
		return ProbeOutcome{}, err
	}
	if s.prober == nil {
		return ProbeOutcome{}, domain.NewInternalError("connectivity testing is unavailable")
	}
	probeCtx, cancel := context.WithTimeout(ctx, connectivityProbeTimeout)
	defer cancel()

	outcome, err := s.prober.ProbeNode(probeCtx, node, strings.TrimSpace(credential))
	if err != nil {
		outcome = ProbeOutcome{State: domain.EndpointTestFail, Message: "the connectivity test could not run"}
	}
	outcome.State = normalizedTestState(outcome.State)
	return outcome, nil
}

// rejectPrefixCollision refuses a prefix that already resolves in the registry or
// belongs to another node.
//
// ignoreID is the node being updated: its own current prefix must not be treated as
// a collision with itself. The registry check runs first because it covers both the
// built-in entries and any custom node the runtime index already carries, which is
// why the index is an interface rather than a fixed value.
func (s *NodeService) rejectPrefixCollision(ctx context.Context, node domain.ProviderNode, ignoreID string) error {
	if _, taken := s.index.Provider(node.Prefix()); taken {
		return domain.ErrNodePrefixTaken
	}
	nodes, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, other := range nodes {
		if other.ID() == ignoreID || other.ID() == node.ID() {
			continue
		}
		if strings.EqualFold(other.Prefix(), node.Prefix()) {
			return domain.ErrNodePrefixTaken
		}
	}
	return nil
}

// NodeCustomNode renders a stored node in the shape the registry needs to
// synthesize its provider entry, so the runtime index can include custom nodes
// without the registry package importing the domain.
func NodeCustomNode(node domain.ProviderNode) registry.CustomNode {
	return registry.CustomNode{
		ID:      node.ID(),
		Name:    node.Name(),
		Prefix:  node.Prefix(),
		APIType: node.APIType(),
		BaseURL: node.BaseURL(),
	}
}
