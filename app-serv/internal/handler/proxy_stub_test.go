// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/proxy_stub_test.go
// @for       The in-memory proxy store and prober the §7.11 route tests drive.
// @uses      internal/domain, internal/service, context, sort, strings, testing.
// @reason    The handler takes a concrete *service.ProxyService, so the route
//
//	tests build the real service over doubles rather than faking the
//	service itself — the seam the production wiring uses is the same
//	one the tests use.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// stubProxyRepo is an in-memory repository.ProxyRepository.
type stubProxyRepo struct {
	byID map[string]domain.Proxy
}

func newStubProxyRepo() *stubProxyRepo {
	return &stubProxyRepo{byID: map[string]domain.Proxy{}}
}

func (r *stubProxyRepo) Create(_ context.Context, proxy domain.Proxy) error {
	r.byID[proxy.ID()] = proxy
	return nil
}

func (r *stubProxyRepo) List(_ context.Context) ([]domain.Proxy, error) {
	out := make([]domain.Proxy, 0, len(r.byID))
	for _, proxy := range r.byID {
		out = append(out, proxy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label() < out[j].Label() })
	return out, nil
}

func (r *stubProxyRepo) GetByID(_ context.Context, id string) (domain.Proxy, error) {
	proxy, ok := r.byID[id]
	if !ok {
		return domain.Proxy{}, domain.ErrProxyNotFound
	}
	return proxy, nil
}

func (r *stubProxyRepo) Update(_ context.Context, proxy domain.Proxy) error {
	if _, ok := r.byID[proxy.ID()]; !ok {
		return domain.ErrProxyNotFound
	}
	r.byID[proxy.ID()] = proxy
	return nil
}

func (r *stubProxyRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrProxyNotFound
	}
	delete(r.byID, id)
	return nil
}

// stubProxyProber records the targets it was asked about and answers with one
// configured result.
type stubProxyProber struct {
	result  service.ProxyProbeResult
	targets []service.ProxyTarget
}

func (p *stubProxyProber) ProbeProxy(_ context.Context, target service.ProxyTarget) (service.ProxyProbeResult, error) {
	p.targets = append(p.targets, target)
	return p.result, nil
}

// newStubProxyService builds the §7.11 service over the doubles. A nil prober
// gets a healthy one, so a CRUD case does not have to say what it is not
// testing.
func newStubProxyService(t *testing.T, prober *stubProxyProber) (*service.ProxyService, *stubProxyRepo) {
	t.Helper()
	repo := newStubProxyRepo()
	if prober == nil {
		prober = &stubProxyProber{result: service.ProxyProbeResult{State: domain.EndpointTestOK, LatencyMS: 7}}
	}
	sealer, err := domain.NewSealer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewSealer() error = %v", err)
	}
	svc, err := service.NewProxyService(service.ProxyServiceDeps{Repo: repo, Sealer: sealer, Prober: prober})
	if err != nil {
		t.Fatalf("NewProxyService() error = %v", err)
	}
	return svc, repo
}
