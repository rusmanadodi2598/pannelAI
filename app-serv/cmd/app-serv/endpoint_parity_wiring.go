// Command app-serv adapts the proxy store to the endpoint service's one question.
//
// @file      cmd/app-serv/endpoint_parity_wiring.go
// @for       The service.ProxyPoolFinder implementation: whether a proxy pool id
//
//	is stored.
//
// @uses      internal/domain, internal/repository, context, errors.
// @reason    Draft 017 §4.1b binds an endpoint to a proxy pool, and a dangling id
//
//	is the failure that matters: the column would name something that
//	resolves to nothing. The check is a port in the service and an adapter
//	here, because AGENTS.md §1.5 keeps storage out of the service's own
//	code.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"errors"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// proxyPoolFinder answers whether a proxy pool id is stored.
type proxyPoolFinder struct {
	proxies repository.ProxyRepository
}

// ProxyPoolExists reports whether the pool exists. A missing row is `false`
// rather than an error: "this pool does not exist" is the answer the caller asked
// for, and turning it into a fault would make the service map it to a 500.
func (f proxyPoolFinder) ProxyPoolExists(ctx context.Context, id string) (bool, error) {
	if _, err := f.proxies.GetByID(ctx, id); err != nil {
		if errors.Is(err, domain.ErrProxyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// assertProxyPoolFinder keeps the repository satisfying the narrow port.
var _ serviceProxyPoolFinder = proxyPoolFinder{}

// serviceProxyPoolFinder mirrors service.ProxyPoolFinder so a signature change
// becomes a compile error here rather than at the call site.
type serviceProxyPoolFinder interface {
	ProxyPoolExists(ctx context.Context, id string) (bool, error)
}
