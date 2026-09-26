// Command app-serv wires the process-wide egress policy.
//
// @file      cmd/app-serv/egress_wiring.go
// @for       Builds the one egress guard, the guarded HTTP client every
//
//	upstream dial shares, and the proxy route it selects per request.
//
// @uses      internal/config, internal/dataplane, internal/domain, internal/netguard,
//
//	context, fmt, net/http, net/url, strings, time.
//
// @reason    OWASP A01 makes the outbound policy one decision rather than one
//
//	per caller: the connectivity probe, the chat and media transports,
//	the OAuth client, and the proxy test all dial an address an operator
//	typed, so they must share one guard and one allowlist. Building it
//	here is what keeps a second allowlist from appearing — the proxy
//	wiring was the first caller and used to build its own.
//
//	SPEC-API-001 §7.11 makes the routing half of that decision here too:
//	settings.network.outbound_proxy_* is read per request, so the guard
//	and the route are wired together and a proxied call cannot skip the
//	destination check.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-19
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/config"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
)

// egressKeepAlive mirrors the keep-alive the plain client configures, so a
// guarded dialer keeps the connection behaviour the pool was tuned for.
const egressKeepAlive = 30 * time.Second

// networkSettingsReader is the one settings read the route needs. It is an
// interface so the wiring names the method it uses rather than the service that
// happens to implement it.
type networkSettingsReader interface {
	Settings(ctx context.Context) (domain.Settings, error)
}

// egress is the process-wide outbound policy.
type egress struct {
	// Guard validates a destination: before the dial wherever a caller can name
	// the host (the probe, the proxy test), and at connect time everywhere.
	Guard *netguard.Guard
	// Client is the one HTTP client every upstream call is made with.
	Client *http.Client
}

// buildEgress parses EGRESS_ALLOWED_TARGETS once and returns the guard plus the
// client built on its dialer. A malformed entry fails the boot rather than
// silently widening what the gateway may reach.
func buildEgress(cfg config.Config, settings networkSettingsReader) (egress, error) {
	guard, err := netguard.NewGuard(cfg.EgressAllowedTargets)
	if err != nil {
		return egress{}, fmt.Errorf("egress wiring: %w", err)
	}
	return egress{
		Guard: guard,
		Client: dataplane.NewHTTPClient(dataplane.HTTPClientDeps{
			Dialer: guard.NewDialer(dataplane.ConnectTimeout, egressKeepAlive),
			Proxy:  egressProxy(guard, settings),
		}),
	}, nil
}

// egressProxy builds the route selector the shared client calls once per
// request (G4, SPEC-API-001 §7.11).
//
// Two rules make it more than a lookup:
//
//   - A proxied request still has its destination validated. The dialer's guard
//     sees the proxy's address, never the destination's, so without this check
//     enabling a proxy would switch the A01 policy off for every call.
//   - A settings read or a proxy URL that fails refuses the request instead of
//     dialing direct. Quietly bypassing a proxy an operator enabled is the
//     failure this setting exists to prevent.
//
// The settings are read per request — the same per-call rule the §7.10 media
// override follows — so a change takes effect on the next call, not the next
// boot.
func egressProxy(guard *netguard.Guard, settings networkSettingsReader) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		ctx := req.Context()
		document, err := settings.Settings(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading the outbound proxy settings: %w", err)
		}
		host := req.URL.Hostname()
		route, err := proxyRoute(document.Network, host)
		if err != nil || route == nil {
			return nil, err
		}
		if err := guard.CheckHost(ctx, host); err != nil {
			return nil, err
		}
		return route, nil
	}
}

// proxyRoute resolves settings.network into the route for one destination, or
// nil when the call goes direct: the proxy is off, no URL is configured, or the
// host is exempt.
func proxyRoute(network domain.NetworkSettings, host string) (*url.URL, error) {
	if !network.OutboundProxyEnabled {
		return nil, nil
	}
	raw := strings.TrimSpace(network.OutboundProxyURL)
	if raw == "" || domain.ProxyExempt(network.OutboundNoProxy, host) {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("egress wiring: outbound_proxy_url %q is not a usable proxy URL", raw)
	}
	return parsed, nil
}
