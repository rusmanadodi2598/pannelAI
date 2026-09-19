// Command app-serv wires the process-wide egress policy.
//
// @file      cmd/app-serv/egress_proxy_route_test.go
// @for       The routing decision itself: settings.network and one host in, a
//
//	route or a refusal out.
//
// @uses      testing, net/url, internal/domain.
// @reason    The server-backed rows in egress_proxy_test.go prove the setting
//
//	reaches the transport; this table proves the rule those rows rest on,
//	including the no-proxy spellings a person is most likely to get wrong
//	(a bare suffix that must not match a longer label).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"net/url"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestProxyRoute pins the routing decision itself, including the spellings the
// no-proxy list accepts.
func TestProxyRoute(t *testing.T) {
	const proxyURL = "http://proxy.test:3128"
	cases := []struct {
		name    string
		network domain.NetworkSettings
		host    string
		want    string
		wantErr bool
	}{
		{name: "disabled", host: "api.example.com"},
		{name: "enabled without a URL", network: domain.NetworkSettings{OutboundProxyEnabled: true}},
		{
			name: "enabled",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxyURL,
			},
			host: "api.example.com", want: proxyURL,
		},
		{
			name: "the exact host is exempt",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxyURL,
				OutboundNoProxy: "api.example.com",
			},
			host: "api.example.com",
		},
		{
			name: "a dotted suffix is exempt",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxyURL,
				OutboundNoProxy: ".example.com",
			},
			host: "api.example.com",
		},
		{
			name: "a suffix matches only on a label boundary",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxyURL,
				OutboundNoProxy: "example.com",
			},
			host: "notexample.com", want: proxyURL,
		},
		{
			name: "a star exempts everything",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxyURL,
				OutboundNoProxy: "*",
			},
			host: "api.example.com",
		},
		{
			name: "a list is split and trimmed",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxyURL,
				OutboundNoProxy: " other.test , api.example.com ",
			},
			host: "api.example.com",
		},
		{
			name: "a URL without a host is refused",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: "http://",
			},
			host: "api.example.com", wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			route, err := proxyRoute(tc.network, tc.host)
			if tc.wantErr {
				if err == nil {
					t.Fatal("proxyRoute() error = nil, want a malformed proxy URL refused")
				}
				return
			}
			if err != nil {
				t.Fatalf("proxyRoute() error = %v", err)
			}
			if got := routeString(route); got != tc.want {
				t.Fatalf("proxyRoute() = %q, want %q", got, tc.want)
			}
		})
	}
}

// routeString renders a route for comparison, so a nil route reads as "direct".
func routeString(route *url.URL) string {
	if route == nil {
		return ""
	}
	return route.String()
}
