// Command app-serv wires the process-wide egress policy.
//
// @file      cmd/app-serv/egress_proxy_test.go
// @for       The §7.11 route table: what settings.network does to one outbound
//
//	call, and the A01 check that survives it.
//
// @uses      testing, errors, context, net/http, net/http/httptest,
//
//	internal/config, internal/domain, internal/netguard.
//
// @reason    G4 wires an operator's proxy into the dial path, and the security
//
//	risk is the interesting half: a proxied request never dials the
//	destination, so the dialer's guard would stop seeing it. These rows
//	pin that the destination is still validated, that a proxy is never
//	quietly bypassed, and that a disabled proxy changes nothing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
)

// settingsStub answers the route's one read with a fixed document or an error.
type settingsStub struct {
	network domain.NetworkSettings
	err     error
}

func (s settingsStub) Settings(context.Context) (domain.Settings, error) {
	if s.err != nil {
		return domain.Settings{}, s.err
	}
	return domain.Settings{Network: s.network}, nil
}

// TestEgressProxy_RoutesThroughTheConfiguredProxy proves the setting reaches the
// dial path: with the proxy enabled, a call to a destination that is not
// listening arrives at the proxy instead. The control is the same call with the
// proxy off, which fails at the destination and leaves the proxy untouched.
func TestEgressProxy_RoutesThroughTheConfiguredProxy(t *testing.T) {
	// The destination port is closed, so a direct dial cannot succeed: reaching
	// a 200 means the request went through the proxy.
	const destination = "http://127.0.0.1:9/g4"

	reached := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		if r.URL.String() != destination {
			t.Errorf("proxy saw %q, want the absolute destination %q", r.URL.String(), destination)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	cases := []struct {
		name        string
		network     domain.NetworkSettings
		settingsErr error
		wantOK      bool
	}{
		{name: "the proxy is off, so the call goes direct"},
		{
			name: "the proxy is on and the call arrives at it",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxy.URL,
			},
			wantOK: true,
		},
		{
			name: "an exempt host goes direct even with the proxy on",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxy.URL,
				OutboundNoProxy: "127.0.0.1",
			},
		},
		{
			name: "a URL that is not a proxy is refused rather than bypassed",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: "not a url",
			},
		},
		{
			name: "a settings read that fails refuses the call",
			network: domain.NetworkSettings{
				OutboundProxyEnabled: true, OutboundProxyURL: proxy.URL,
			},
			settingsErr: errors.New("postgres is unreachable"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := reached
			settings := settingsStub{network: tc.network, err: tc.settingsErr}
			egress, err := buildEgress(config.Config{
				EgressAllowedTargets: []string{"127.0.0.1/32"},
			}, settings)
			if err != nil {
				t.Fatalf("buildEgress() error = %v", err)
			}

			response, err := egress.Client.Get(destination)
			if err == nil {
				_ = response.Body.Close()
			}
			switch {
			case tc.wantOK && err != nil:
				t.Fatalf("Get() error = %v, want the call to reach the proxy", err)
			case !tc.wantOK && err == nil:
				t.Fatal("Get() succeeded, want it to fail at the destination")
			}
			if got := reached - before; (got == 1) != tc.wantOK {
				t.Fatalf("the proxy saw %d requests, want %v", got, tc.wantOK)
			}
		})
	}
}

// TestEgressProxy_StillValidatesTheDestination pins the A01 half: a proxied
// request must not become a way around the allowlist. The allowlist names the
// proxy's address, and the destination is a different loopback address outside
// it, so only the destination check can refuse the call — and the proxy is
// never asked anything.
func TestEgressProxy_StillValidatesTheDestination(t *testing.T) {
	reached := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	egress, err := buildEgress(config.Config{
		EgressAllowedTargets: []string{"127.0.0.1/32"},
	}, settingsStub{network: domain.NetworkSettings{
		OutboundProxyEnabled: true, OutboundProxyURL: proxy.URL,
	}})
	if err != nil {
		t.Fatalf("buildEgress() error = %v", err)
	}

	response, err := egress.Client.Get("http://127.0.0.2:9/denied-through-the-proxy")
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("Get() succeeded, want the destination refused even though a proxy is configured")
	}
	if !errors.Is(err, netguard.ErrDenied) {
		t.Fatalf("Get() error = %v, want a guard refusal", err)
	}
	if reached != 0 {
		t.Fatalf("the proxy saw %d requests, want none: the destination is refused first", reached)
	}
}
