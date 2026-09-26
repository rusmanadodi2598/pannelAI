// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/proxy_rotation_test.go
// @for       Tests for the proxy route plan vocabulary: the strategy's closed
//
//	set, the usability rule, and the dial URL a candidate dials with.
//
// @uses      internal/domain, net/url, testing, time.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D2/D3: the strategy is a closed
//
//	set whose empty stored value reads as the default, usability is the
//	aggregate's own rule (enabled, and the last probe is not a failure),
//	and the dial URL is the aggregate's knowledge because only it knows
//	how protocol, host, port, and the opened secret compose. Pinning all
//	three here keeps the service a coordinator rather than a second
//	implementation of the rules.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package domain

import (
	"strings"
	"testing"
	"time"
)

func TestParseProxyStrategy(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "an empty stored value reads as the default", value: "", want: ProxyStrategyFallback},
		{name: "fallback", value: "fallback", want: ProxyStrategyFallback},
		{name: "round_robin", value: "round_robin", want: ProxyStrategyRoundRobin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseProxyStrategy(tc.value)
			if err != nil {
				t.Fatalf("ParseProxyStrategy(%q) error = %v", tc.value, err)
			}
			if got != tc.want {
				t.Fatalf("ParseProxyStrategy(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestParseProxyStrategy_Refusals(t *testing.T) {
	// The reference's third option ("random") is deliberately not shipped:
	// a closed set of two is what the panel can render and the contract can
	// declare, and an unknown spelling must be refused rather than defaulted.
	cases := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "the reference's random", value: "random", wantErr: "outbound_proxy_strategy"},
		{name: "uppercase", value: "FALLBACK", wantErr: "outbound_proxy_strategy"},
		{name: "untrimmed", value: " round_robin", wantErr: "outbound_proxy_strategy"},
		{name: "a strategy-like word", value: "none", wantErr: "outbound_proxy_strategy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseProxyStrategy(tc.value)
			if err == nil {
				t.Fatalf("ParseProxyStrategy(%q) accepted an outside value", tc.value)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ParseProxyStrategy(%q) error = %v, want it to mention %q", tc.value, err, tc.wantErr)
			}
		})
	}
}

func routeProxy(t *testing.T, enabled bool, state string) Proxy {
	t.Helper()
	proxy, err := NewProxy("", "pool", ProxyProtocolHTTP, "proxy.example.com", 8080, "", "", time.Now())
	if err != nil {
		t.Fatalf("NewProxy() error = %v", err)
	}
	proxy.SetEnabled(enabled, time.Now())
	if state != "" {
		proxy.RecordTest(state, 42, "", time.Now())
	}
	return proxy
}

func TestProxyRouteUsable(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		state   string
		want    bool
	}{
		{name: "enabled and never tested", enabled: true, state: "", want: true},
		{name: "enabled and proven ok", enabled: true, state: "ok", want: true},
		{name: "enabled but the last probe failed", enabled: true, state: "fail", want: false},
		{name: "enabled with a state the panel does not know", enabled: true, state: "degraded", want: true},
		{name: "disabled is never usable", enabled: false, state: "ok", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := routeProxy(t, tc.enabled, tc.state)
			if got := proxy.RouteUsable(); got != tc.want {
				t.Fatalf("RouteUsable(enabled=%v, state=%q) = %v, want %v", tc.enabled, tc.state, got, tc.want)
			}
		})
	}
}

func TestProxyDialURL(t *testing.T) {
	cases := []struct {
		name     string
		protocol ProxyProtocol
		host     string
		port     int
		username string
		secret   string
		want     string
	}{
		{
			name: "plain http", protocol: ProxyProtocolHTTP, host: "proxy.example.com", port: 8080,
			want: "http://proxy.example.com:8080",
		},
		{
			name: "credentials ride the userinfo", protocol: ProxyProtocolHTTP, host: "proxy.example.com", port: 8080,
			username: "operator", secret: "s3cret", want: "http://operator:s3cret@proxy.example.com:8080",
		},
		{
			name: "special characters are escaped", protocol: ProxyProtocolHTTP, host: "proxy.example.com", port: 8080,
			username: "operator", secret: "p@ss:wo/rd", want: "http://operator:p%40ss%3Awo%2Frd@proxy.example.com:8080",
		},
		{
			name: "socks5", protocol: ProxyProtocolSOCKS5, host: "10.0.0.8", port: 1080,
			want: "socks5://10.0.0.8:1080",
		},
		{
			name: "https", protocol: ProxyProtocolHTTPS, host: "secure.example.com", port: 8443,
			want: "https://secure.example.com:8443",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy, err := NewProxy("", "pool", tc.protocol, tc.host, tc.port, tc.username, "sealed", time.Now())
			if err != nil {
				t.Fatalf("NewProxy() error = %v", err)
			}
			got, err := proxy.DialURL(tc.secret)
			if err != nil {
				t.Fatalf("DialURL() error = %v", err)
			}
			if got.String() != tc.want {
				t.Fatalf("DialURL() = %q, want %q", got.String(), tc.want)
			}
		})
	}
}

func TestProxyDialURL_Refusals(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		port    int
		wantErr string
	}{
		{name: "no host", host: "", port: 8080, wantErr: "host"},
		{name: "no port", host: "proxy.example.com", port: 0, wantErr: "port"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// RehydrateProxy is the repository load path and skips the
			// constructor's validation, so DialURL defends itself: a row the
			// store cannot explain must refuse to dial rather than compose a
			// URL that points nowhere.
			proxy := RehydrateProxy("prx_raw", "pool", ProxyProtocolHTTP, tc.host, tc.port, "", "", true, ProxyTestStatus{}, time.Now(), time.Now())
			_, err := proxy.DialURL("")
			if err == nil {
				t.Fatalf("DialURL(host=%q, port=%d) accepted a shape that must be refused", tc.host, tc.port)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("DialURL() error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}
