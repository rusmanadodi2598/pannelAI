// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_probe.go
// @for       The two proxy connectivity tests of SPEC-API-001 §7.11.
// @uses      internal/domain, context.
// @reason    §7.11 offers a stored test and a candidate test, and both must
//
//	answer with the same normalized state — a failed probe is a result,
//	not an error. They are separate from the CRUD use cases because
//	AGENTS.md §1.1 caps a file at 250 lines and the two paths only share
//	the sealing helpers.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Test probes a stored candidate and stores what it found, so the panel's list
// and the route's answer report the same result.
func (s *ProxyService) Test(ctx context.Context, id string) (domain.ProxyTestStatus, error) {
	proxy, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.ProxyTestStatus{}, err
	}
	password, err := s.open(proxy)
	if err != nil {
		return domain.ProxyTestStatus{}, err
	}
	status, err := s.probe(ctx, ProxyTarget{
		Protocol: proxy.Protocol(), Host: proxy.Host(), Port: proxy.Port(),
		Username: proxy.Username(), Password: password,
	})
	if err != nil {
		return domain.ProxyTestStatus{}, err
	}
	proxy.RecordTest(status.State, status.LatencyMS, status.Message, s.clock().UTC())
	if err := s.repo.Update(ctx, proxy); err != nil {
		return domain.ProxyTestStatus{}, err
	}
	return proxy.Status(), nil
}

// TestCandidate probes an unsaved candidate, which is what the panel uses while
// the operator is still filling the form.
func (s *ProxyService) TestCandidate(ctx context.Context, target ProxyTarget) (domain.ProxyTestStatus, error) {
	return s.probe(ctx, target)
}

// probe runs one test and normalizes its state.
//
// Any state that is not a success is recorded as a failure: a result this layer
// does not recognize must not read as a pass (OWASP A10, fail closed).
func (s *ProxyService) probe(ctx context.Context, target ProxyTarget) (domain.ProxyTestStatus, error) {
	result, err := s.prober.ProbeProxy(ctx, target)
	if err != nil {
		return domain.ProxyTestStatus{}, err
	}
	state := domain.EndpointTestFail
	if result.State == domain.EndpointTestOK {
		state = domain.EndpointTestOK
	}
	checked := s.clock().UTC()
	return domain.ProxyTestStatus{
		State: state, LatencyMS: result.LatencyMS, CheckedAt: &checked, Message: result.Message,
	}, nil
}
