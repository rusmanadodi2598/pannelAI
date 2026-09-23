// Command app-serv adapts the connectivity probe port to HTTP.
//
// @file      cmd/app-serv/provider_probe_fixture_test.go
// @for       The shared fixtures the two probe test files build on: the egress
//
//	guard, an endpoint, and a key.
//
// @uses      testing, time, internal/domain, internal/netguard.
// @reason    The endpoint-probe and node-probe suites both need a guard and an
//
//	endpoint, and AGENTS.md §1.1 caps a file at 250 lines: the fixtures
//	live here so neither suite carries a copy that can drift from the
//	other, and so the loopback rule is stated once.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
)

// probeGuard builds the guard the probe tests run under. Loopback is refused
// unless it is named, so a test that wants to reach its httptest server must
// allowlist it — which is the rule a self-hosted deployment follows.
func probeGuard(t *testing.T, allowed ...string) *netguard.Guard {
	t.Helper()
	guard, err := netguard.NewGuard(allowed)
	if err != nil {
		t.Fatalf("netguard.NewGuard(%v) error = %v", allowed, err)
	}
	return guard
}

// newEndpoint builds an endpoint for the probe tests.
func newEndpoint(t *testing.T, providerID string) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(
		"ep_probe", providerID, "primary", domain.UpstreamAuthAPIKey, 1, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewUpstreamEndpoint() error = %v", err)
	}
	return endpoint
}

// newKey builds a key for the probe tests.
func newKey(t *testing.T, endpointID string) domain.UpstreamKey {
	t.Helper()
	key, err := domain.NewUpstreamKey(
		"primary", endpointID, "uky_probe", "v1:sealed", "sk-…robe", 1, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewUpstreamKey() error = %v", err)
	}
	return key
}
