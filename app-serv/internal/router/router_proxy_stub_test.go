// Package router maps HTTP routes to handlers.
//
// @file      internal/router/router_proxy_stub_test.go
// @for       The in-memory proxy store and prober the §7.11 route tests drive.
// @uses      internal/domain, internal/handler, internal/service, context, sort,
//
//	strings, testing.
//
// @reason    AGENTS.md §1.1 caps a file at 250 lines and the route table plus
//
//	its cases already fills one; the doubles are a separate concern, and
//	keeping them here means the route file reads as the audit table it is.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     router
// @stability experimental
// @since     2026-09-19
package router

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// newProxyRouteHandler wires the §7.11 service over the in-memory store and a
// prober that always answers healthy.
func newProxyRouteHandler(t *testing.T) *handler.ProxyHandler {
	t.Helper()
	sealer, err := domain.NewSealer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	svc, err := service.NewProxyService(service.ProxyServiceDeps{
		Repo: newMemProxyRepo(), Sealer: sealer, Prober: routeProxyProber{},
	})
	if err != nil {
		t.Fatalf("NewProxyService() error = %v", err)
	}
	return handler.NewProxyHandler(svc)
}

// routeProxyProber answers every connectivity test healthily, so the route
// table can drive both test routes without a network.
type routeProxyProber struct{}

func (routeProxyProber) ProbeProxy(context.Context, service.ProxyTarget) (service.ProxyProbeResult, error) {
	return service.ProxyProbeResult{State: domain.EndpointTestOK, LatencyMS: 7}, nil
}

// memProxyRepo implements repository.ProxyRepository in memory.
type memProxyRepo struct {
	byID map[string]domain.Proxy
}

func newMemProxyRepo() *memProxyRepo {
	return &memProxyRepo{byID: map[string]domain.Proxy{}}
}

func (r *memProxyRepo) Create(_ context.Context, proxy domain.Proxy) error {
	r.byID[proxy.ID()] = proxy
	return nil
}

func (r *memProxyRepo) List(context.Context) ([]domain.Proxy, error) {
	out := make([]domain.Proxy, 0, len(r.byID))
	for _, proxy := range r.byID {
		out = append(out, proxy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label() < out[j].Label() })
	return out, nil
}

func (r *memProxyRepo) GetByID(_ context.Context, id string) (domain.Proxy, error) {
	proxy, ok := r.byID[id]
	if !ok {
		return domain.Proxy{}, domain.ErrProxyNotFound
	}
	return proxy, nil
}

func (r *memProxyRepo) Update(_ context.Context, proxy domain.Proxy) error {
	if _, ok := r.byID[proxy.ID()]; !ok {
		return domain.ErrProxyNotFound
	}
	r.byID[proxy.ID()] = proxy
	return nil
}

func (r *memProxyRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrProxyNotFound
	}
	delete(r.byID, id)
	return nil
}
