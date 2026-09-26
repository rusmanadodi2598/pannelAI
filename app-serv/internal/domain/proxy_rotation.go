// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/proxy_rotation.go
// @for       The proxy route plan vocabulary: the rotation strategy's closed
//
//	set, a candidate's usability rule, and the dial URL it composes.
//
// @uses      internal/domain (errors, Proxy), net/url, strings.
// @reason    docs/PORT/008-PORT-PROXY-ENGINE.md D2/D3: the strategy is a closed
//
//	set whose empty stored value reads as the default, usability is the
//	aggregate's own rule (enabled, and the last probe is not a failure),
//	and the dial URL is the aggregate's knowledge because only it knows
//	how protocol, host, port, and the opened secret compose. The
//	round-robin order itself reuses RotateRefs, so the distribution rule
//	stays the one implementation the combo rotation already pins.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The strategy set the engine ships (docs/PORT/008-PORT-PROXY-ENGINE.md D2).
// The reference's "random" is deliberately absent: a closed set of two is what
// the panel can render and the contract can declare.
const (
	ProxyStrategyFallback   = "fallback"
	ProxyStrategyRoundRobin = "round_robin"
)

// DefaultProxyStrategy is what an empty stored strategy reads as (D2): a
// deployment that stored its settings before the key existed keeps a valid
// document, and fallback is the safe reading of an unstated preference.
const DefaultProxyStrategy = ProxyStrategyFallback

// ProxyFailureCooldown is how long one connect-stage failure parks a candidate
// (D6). Two minutes rides out a dead tunnel without burying a recovering
// proxy for minutes on end; the candidate's own Test button is the operator's
// faster lever.
const ProxyFailureCooldown = 2 * time.Minute

// MaxProxyRouteAttempts bounds the failover walk per request (D7): a pool of
// twenty dead candidates must not turn one chat call into twenty connect
// timeouts. The plan may carry more entries than this; the transport walks the
// first three and returns the last error.
const MaxProxyRouteAttempts = 3

// ProxyRoutePoolKey is the rotation cursor's pool name (D5): v1 routes one
// global pool, so one counter serves every destination.
const ProxyRoutePoolKey = "global"

// ProxyExempt reports whether a destination skips the proxy. The list is
// comma-separated; `*` exempts everything, and an entry matches the host
// exactly or as a domain suffix, which is the spelling NO_PROXY uses.
//
// It lives in the domain because two routes must read one rule: the static
// URL's per-request selector and the pool plan refuse the same destinations,
// and a second implementation would let them drift.
func ProxyExempt(list, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, entry := range strings.Split(list, ",") {
		entry = strings.ToLower(strings.Trim(strings.TrimSpace(entry), "."))
		if entry == "" {
			continue
		}
		if entry == "*" || host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}

// ProxyRouteAttempt is one dial the plan offers the transport: the pool row the
// attempt belongs to (so a failure can be reported against it) and the proxy
// URL to dial through.
type ProxyRouteAttempt struct {
	ID  string
	URL *url.URL
}

// ParseProxyStrategy validates the closed set. An empty value reads as the
// default (D2); any other spelling is a validation error naming the key, which
// is what the schema boundary reports onward.
func ParseProxyStrategy(value string) (string, error) {
	switch value {
	case "":
		return DefaultProxyStrategy, nil
	case ProxyStrategyFallback, ProxyStrategyRoundRobin:
		return value, nil
	default:
		return "", NewValidationError(
			"outbound_proxy_strategy must be fallback or round_robin")
	}
}

// RouteUsable reports whether the candidate may serve egress traffic
// (docs/PORT/008-PORT-PROXY-ENGINE.md D3): the operator's own switch is on and
// the last probe is not a failure. A row never tested is usable, because
// routing before testing is the operator's call to make; a failed probe is
// evidence, and the row stays out of the plan until a re-test replaces it.
func (p Proxy) RouteUsable() bool {
	return p.enabled && p.status.State != "fail"
}

// DialURL composes the URL an attempt dials through: the candidate's protocol,
// host, and port with the opened secret riding the userinfo. The secret never
// leaves the process in any other form, and special characters are escaped by
// url.UserPassword rather than spliced into the string.
//
// The constructor validates host and port, but the repository's rehydrate path
// bypasses it, so this method defends itself: a row the store cannot explain
// refuses to dial rather than composing a URL that points nowhere.
func (p Proxy) DialURL(openedPassword string) (*url.URL, error) {
	host := strings.TrimSpace(p.host)
	if host == "" {
		return nil, NewValidationError("proxy host is required")
	}
	if p.port <= 0 {
		return nil, NewValidationError("proxy port is required")
	}
	composed := &url.URL{
		Scheme: string(p.protocol),
		Host:   host + ":" + strconv.Itoa(p.port),
	}
	if p.username != "" {
		composed.User = url.UserPassword(p.username, openedPassword)
	}
	return composed, nil
}
