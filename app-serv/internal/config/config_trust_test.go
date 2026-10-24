// Package config reads process settings into a typed, validated struct.
//
// @file      internal/config/config_trust_test.go
// @for       The egress allowlist and trusted-proxy list parsed at boot.
// @uses      net, strings, testing.
// @reason    A malformed trust entry must fail the boot rather than install a set the limiters cannot read, and the allowlist split rules are the egress guard's input; these two stay together because both decide who may reach whom.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-10-24
package config

import (
	"net"
	"strings"
	"testing"
)

// TestLoad_EgressAllowedTargets pins the allowlist parsing: entries are split
// on commas, trimmed, and blanks dropped, so a trailing comma is not an entry
// of "", an empty entry would reach the guard as a prefix it cannot parse.
func TestLoad_EgressAllowedTargets(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "unset", raw: "", want: []string{}},
		{name: "one address", raw: "10.0.0.5", want: []string{"10.0.0.5"}},
		{name: "a prefix and a host", raw: "10.0.0.0/8, proxy.internal", want: []string{"10.0.0.0/8", "proxy.internal"}},
		{name: "blanks dropped", raw: " 10.0.0.5 , ,", want: []string{"10.0.0.5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			if tc.raw != "" {
				env["EGRESS_ALLOWED_TARGETS"] = tc.raw
			}
			setEnv(t, env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if strings.Join(cfg.EgressAllowedTargets, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("EgressAllowedTargets = %v, want %v", cfg.EgressAllowedTargets, tc.want)
			}
		})
	}
}

// TestLoad_TrustedProxyCIDRs pins the parse of the limiter trust set: unset
// trusts no proxy, a bare address becomes a host route, and a malformed entry
// fails the boot rather than installing an unreadable set.
func TestLoad_TrustedProxyCIDRs(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantErr    bool
		want       int
		wantInside string
	}{
		{name: "unset trusts no proxy", raw: "", want: 0},
		{name: "one prefix", raw: "10.0.0.0/8", want: 1, wantInside: "10.9.9.9"},
		{name: "a bare address becomes a host route", raw: "10.0.0.5", want: 1, wantInside: "10.0.0.5"},
		{name: "malformed entry fails the boot", raw: "not-a-cidr", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			if tc.raw != "" {
				env["TRUSTED_PROXY_CIDRS"] = tc.raw
			}
			setEnv(t, env)
			cfg, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatal("Load() error = nil, want the malformed entry refused")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(cfg.TrustedProxies) != tc.want {
				t.Fatalf("TrustedProxies = %d nets, want %d", len(cfg.TrustedProxies), tc.want)
			}
			if tc.wantInside != "" && !cfg.TrustedProxies[0].Contains(net.ParseIP(tc.wantInside)) {
				t.Fatalf("trusted net = %v, want it to contain %s", cfg.TrustedProxies[0], tc.wantInside)
			}
		})
	}
}
