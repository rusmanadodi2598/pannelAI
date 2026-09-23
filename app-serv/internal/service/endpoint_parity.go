// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_parity.go
// @for       The one rule the connection-parity fields need beyond the
//
//	aggregate's own validation: a proxy binding must name a stored pool.
//
// @uses      internal/domain, context, strings.
// @reason    Draft 017 §4.1b adds `proxy_pool_id` to an endpoint, and a dangling
// //
//
//	id is the failure mode that matters: the column would hold a name that
//	resolves to nothing, so an operator who deleted a pool would leave
//	endpoints pointing at it with no error anywhere. The check lives here
//	rather than in the domain because the proxy table is another aggregate,
//	and a domain type must not read a repository (AGENTS.md §1.5).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ProxyPoolFinder reports whether a proxy pool id is stored.
//
// It is a one-method port rather than the whole proxy repository so the endpoint
// service depends on the question it asks, and a deployment that wires no proxy
// store refuses the field rather than accepting a dangling id.
type ProxyPoolFinder interface {
	ProxyPoolExists(ctx context.Context, id string) (bool, error)
}

// applyRouting applies the connection-parity part of a patch.
//
// It runs after the label/priority/status edit and before the store write, so a
// refused routing value leaves the whole patch unapplied rather than half of it:
// the aggregate validates before it mutates, and the proxy check runs first
// because it needs a repository read.
func (s *EndpointService) applyRouting(ctx context.Context, endpoint *domain.UpstreamEndpoint, patch UpdatePatch) error {
	if patch.DefaultModel == nil && patch.GlobalPriority == nil && patch.ProxyPoolID == nil {
		return nil
	}
	globalPriority := endpoint.GlobalPriority()
	if patch.GlobalPriority != nil {
		globalPriority = *patch.GlobalPriority
	}
	defaultModel := endpoint.DefaultModel()
	if patch.DefaultModel != nil {
		defaultModel = *patch.DefaultModel
	}
	proxyPoolID := endpoint.ProxyPoolID()
	if patch.ProxyPoolID != nil {
		proxyPoolID = *patch.ProxyPoolID
	}
	if err := s.checkProxyPool(ctx, proxyPoolID); err != nil {
		return err
	}
	return endpoint.SetRouting(globalPriority, defaultModel, proxyPoolID, s.clock())
}

// checkProxyPool refuses a proxy binding that names no stored pool.
//
// An empty id is always accepted: it means "dial directly", which is the state
// every endpoint starts in. A nil finder refuses a non-empty id, because a
// deployment that cannot check must not store a name nothing can resolve.
func (s *EndpointService) checkProxyPool(ctx context.Context, proxyPoolID string) error {
	id := strings.TrimSpace(proxyPoolID)
	if id == "" {
		return nil
	}
	if s.proxies == nil {
		return domain.NewValidationError("proxy_pool_id cannot be set because no proxy store is configured")
	}
	exists, err := s.proxies.ProxyPoolExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return domain.NewValidationError("proxy_pool_id does not name a stored proxy pool")
	}
	return nil
}
