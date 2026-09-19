// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_stub_test.go
// @for       The in-memory proxy repository, prober, and fixture the §7.11
//
//	tests drive.
//
// @uses      context, sort, testing, internal/domain.
// @reason    The store and the dialer are the two things the service must not
//
//	own, so both are doubles here; keeping them with the fixture leaves
//	each test about the rule it pins.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"sort"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubProxyRepo is an in-memory ProxyRepository.
type stubProxyRepo struct {
	byID    map[string]domain.Proxy
	updates int
	failure error
}

func newStubProxyRepo() *stubProxyRepo {
	return &stubProxyRepo{byID: map[string]domain.Proxy{}}
}

func (r *stubProxyRepo) Create(_ context.Context, proxy domain.Proxy) error {
	if r.failure != nil {
		return r.failure
	}
	r.byID[proxy.ID()] = proxy
	return nil
}

func (r *stubProxyRepo) List(_ context.Context) ([]domain.Proxy, error) {
	if r.failure != nil {
		return nil, r.failure
	}
	out := make([]domain.Proxy, 0, len(r.byID))
	for _, proxy := range r.byID {
		out = append(out, proxy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label() < out[j].Label() })
	return out, nil
}

func (r *stubProxyRepo) GetByID(_ context.Context, id string) (domain.Proxy, error) {
	if r.failure != nil {
		return domain.Proxy{}, r.failure
	}
	proxy, ok := r.byID[id]
	if !ok {
		return domain.Proxy{}, domain.ErrProxyNotFound
	}
	return proxy, nil
}

func (r *stubProxyRepo) Update(_ context.Context, proxy domain.Proxy) error {
	if r.failure != nil {
		return r.failure
	}
	if _, ok := r.byID[proxy.ID()]; !ok {
		return domain.ErrProxyNotFound
	}
	r.updates++
	r.byID[proxy.ID()] = proxy
	return nil
}

func (r *stubProxyRepo) Delete(_ context.Context, id string) error {
	if r.failure != nil {
		return r.failure
	}
	if _, ok := r.byID[id]; !ok {
		return domain.ErrProxyNotFound
	}
	delete(r.byID, id)
	return nil
}

// stubProxyProber records the targets it was asked about and answers with one
// configured result.
type stubProxyProber struct {
	result  ProxyProbeResult
	err     error
	targets []ProxyTarget
}

func (p *stubProxyProber) ProbeProxy(_ context.Context, target ProxyTarget) (ProxyProbeResult, error) {
	p.targets = append(p.targets, target)
	if p.err != nil {
		return ProxyProbeResult{}, p.err
	}
	return p.result, nil
}

// newProxyFixture wires the service over the doubles. A nil prober gets a
// healthy one, so a CRUD test does not have to say what it is not testing.
func newProxyFixture(t *testing.T, prober *stubProxyProber) (*ProxyService, *stubProxyRepo) {
	t.Helper()
	repo := newStubProxyRepo()
	if prober == nil {
		prober = &stubProxyProber{result: ProxyProbeResult{State: domain.EndpointTestOK, LatencyMS: 12}}
	}
	service, err := NewProxyService(ProxyServiceDeps{Repo: repo, Sealer: newTestSealer(t), Prober: prober})
	if err != nil {
		t.Fatalf("NewProxyService() error = %v", err)
	}
	return service, repo
}

// proxyDraft builds a valid draft for the given address.
func proxyDraft(label, host string, port int) ProxyDraft {
	return ProxyDraft{
		Label: label, Protocol: domain.ProxyProtocolHTTP, Host: host, Port: port,
		Username: "operator", Password: "s3cret-value",
	}
}

// TestNewProxyService_RequiresDeps pins the constructor's guard.
func TestNewProxyService_RequiresDeps(t *testing.T) {
	repo := newStubProxyRepo()
	sealer := newTestSealer(t)
	prober := &stubProxyProber{}

	cases := []struct {
		name string
		deps ProxyServiceDeps
	}{
		{name: "no repository", deps: ProxyServiceDeps{Sealer: sealer, Prober: prober}},
		{name: "no sealer", deps: ProxyServiceDeps{Repo: repo, Prober: prober}},
		{name: "no prober", deps: ProxyServiceDeps{Repo: repo, Sealer: sealer}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewProxyService(tc.deps); err == nil {
				t.Fatalf("NewProxyService(%s) = nil error, want a validation failure", tc.name)
			}
		})
	}
}
