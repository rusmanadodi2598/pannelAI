// Command app-serv wires the process-wide egress policy.
//
// @file      cmd/app-serv/egress_wiring_test.go
// @for       The egress wiring's table: what the built client refuses, what the
//
//	allowlist opens, and the boot failure a malformed entry causes.
//
// @uses      testing, errors, net/http, net/http/httptest, net/netip,
//
//	internal/config, internal/netguard.
//
// @reason    The guard's own table pins the range rules; this one pins the
//
//	(wiring) — that the client the data plane actually dials with carries
//	the guard's dialer, so the policy cannot be lost between the two.
//	The benign control matters as much as the refusals: a guard that
//	blocked everything would pass a refusal-only test.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
)

// TestBuildEgress_GuardsUpstreamDials pins the G2 policy at the wiring level:
// the client every upstream call is made with refuses a loopback destination
// unless the operator's allowlist names it, and the guard still permits a
// public address.
func TestBuildEgress_GuardsUpstreamDials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cases := []struct {
		name    string
		allowed []string
		wantOK  bool
	}{
		{name: "loopback is refused by default"},
		{name: "the exact loopback is allowed", allowed: []string{"127.0.0.1/32"}, wantOK: true},
		{name: "a wider loopback prefix is allowed", allowed: []string{"127.0.0.0/8"}, wantOK: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			egress, err := buildEgress(config.Config{EgressAllowedTargets: tc.allowed})
			if err != nil {
				t.Fatalf("buildEgress() error = %v", err)
			}
			response, err := egress.Client.Get(server.URL)
			if err == nil {
				_ = response.Body.Close()
			}
			switch {
			case tc.wantOK && err != nil:
				t.Fatalf("Get() error = %v, want the allowlisted loopback to be reached", err)
			case !tc.wantOK && err == nil:
				t.Fatal("Get() reached a loopback address that is not allowlisted")
			case !tc.wantOK && !errors.Is(err, netguard.ErrDenied):
				t.Fatalf("Get() error = %v, want a guard refusal", err)
			}
		})
	}

	// The benign control: a public address passes, and a private one is refused
	// with its reason rather than accepted.
	egress, err := buildEgress(config.Config{})
	if err != nil {
		t.Fatalf("buildEgress() error = %v", err)
	}
	if err := egress.Guard.CheckIP(netip.MustParseAddr("93.184.216.34")); err != nil {
		t.Fatalf("CheckIP(a public address) error = %v, want allowed", err)
	}
	if err := egress.Guard.CheckIP(netip.MustParseAddr("10.0.0.5")); !errors.Is(err, netguard.ErrDenied) {
		t.Fatalf("CheckIP(a private address) error = %v, want a guard refusal", err)
	}
}

// TestBuildEgress_RefusesAMalformedAllowlist pins that a typo fails the boot
// rather than silently widening what the gateway may reach.
func TestBuildEgress_RefusesAMalformedAllowlist(t *testing.T) {
	if _, err := buildEgress(config.Config{EgressAllowedTargets: []string{"10.0.0.0/33"}}); err == nil {
		t.Fatal("buildEgress() = nil error, want a malformed allowlist to be refused")
	}
}
