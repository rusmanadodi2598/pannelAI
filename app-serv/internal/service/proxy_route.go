// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_route.go
// @for       The proxy route engine's one decision: which proxy candidates a request dials through, in which order (docs/PORT/008-PORT-PROXY-ENGINE.md D1-D6).
// @uses      internal/domain, internal/repository, context, net/url, strings, time.
// @reason    §7.11 makes the pool a set of tested candidates and the owner's directive makes it carry traffic: one service owns the plan so the two dataplane call sites walk one order rather than re-implementing the strategy. Every rule the plan applies (usability, exemption, stable order, rotation, parking, the static last resort) is pinned by proxy_route_test.go, and the degradations are deliberate: an unreadable pool degrades to the shared route when the provider inherits the global setting, while a pinned provider fails closed because the pin is the operator's instruction.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-26
package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// proxyListCacheTTL is how long one pool read is reused. The plan runs once per
// proxied request while the pool changes by hand, so a short window removes the
// per-request read without making a panel edit wait for a restart.
const proxyListCacheTTL = 5 * time.Second

// planSettingsReader is the settings read the plan needs per request.
type planSettingsReader interface {
	Settings(ctx context.Context) (domain.Settings, error)
}

// planSecretOpener decrypts one stored proxy password for the dial URL.
type planSecretOpener interface {
	Open(sealed string) (string, error)
}

// proxyLister is the one read the plan makes of the pool. It is an interface
// so the deps name the method the plan uses rather than the repository that
// happens to implement it.
type proxyLister interface {
	List(ctx context.Context) ([]domain.Proxy, error)
}

// ProxyRouteDeps holds the collaborators the route service needs: the settings and
// pool reads a plan makes, the opener that unseals a candidate's password, the
// rotation and cooldown store, and an optional clock.
type ProxyRouteDeps struct {
	Proxies  proxyLister
	Routes   repository.ProxyRouteStore
	Settings planSettingsReader
	Opener   planSecretOpener
	Clock    func() time.Time
}

// ProxyRouteService plans one request's proxy route from the pool.
type ProxyRouteService struct {
	proxies  proxyLister
	routes   repository.ProxyRouteStore
	settings planSettingsReader
	opener   planSecretOpener
	clock    func() time.Time

	mu          sync.Mutex
	cached      []domain.Proxy
	cachedAt    time.Time
	cachedValid bool
}

// NewProxyRouteService returns the service, defaulting the clock to time.Now when
// deps carry none.
func NewProxyRouteService(deps ProxyRouteDeps) *ProxyRouteService {
	clock := deps.Clock
	if clock == nil {
		clock = time.Now
	}
	return &ProxyRouteService{
		proxies:  deps.Proxies,
		routes:   deps.Routes,
		settings: deps.Settings,
		opener:   deps.Opener,
		clock:    clock,
	}
}

// listProxies reuses one pool read for proxyListCacheTTL. A read failure is
// never cached, so a transient database error is retried on the next request.
func (s *ProxyRouteService) listProxies(ctx context.Context) ([]domain.Proxy, error) {
	s.mu.Lock()
	if s.cachedValid && s.clock().Sub(s.cachedAt) < proxyListCacheTTL {
		rows := s.cached
		s.mu.Unlock()
		return rows, nil
	}
	s.mu.Unlock()

	rows, err := s.proxies.List(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cached = rows
	s.cachedAt = s.clock()
	s.cachedValid = true
	s.mu.Unlock()
	return rows, nil
}

// Plan returns the proxy attempts one request from providerID to destHost walks,
// in order. The provider's binding decides the candidate set before the order:
// `__none__` dials direct, an empty pool follows the global setting, and a named
// pool leads while the remaining usable rows follow. A pin works with the global
// switch off, because it is the operator's explicit instruction for that provider;
// the static URL stays a candidate only with the switch on, because it belongs to
// the global configuration. An empty plan is a decision, not an error: the shared
// client serves the request exactly as it did before this engine existed.
func (s *ProxyRouteService) Plan(ctx context.Context, providerID, destHost string) ([]domain.ProxyRouteAttempt, error) {
	document, err := s.settings.Settings(ctx)
	if err != nil {
		return nil, err
	}
	binding, err := document.Network.ProviderProxyFor(providerID)
	if err != nil {
		return nil, err
	}
	if binding.PoolID == domain.ProxyPoolNone {
		// The provider's own binding exempts it: no pool, no static URL.
		return nil, nil
	}
	if binding.PoolID == "" && !document.Network.OutboundProxyEnabled {
		return nil, nil
	}
	if domain.ProxyExempt(document.Network.OutboundNoProxy, destHost) {
		return nil, nil
	}

	rows, err := s.listProxies(ctx)
	if err != nil {
		// A pin is the operator's instruction, so an unreadable pool refuses the
		// request instead of quietly dialing around it. Without a pin the shared
		// client already carries the static route, and keeping that degradation is
		// what stops a read failure from taking the data plane down.
		if binding.PoolID != "" {
			return nil, fmt.Errorf("reading the proxy pool pinned by %s: %w", providerID, err)
		}
		//nolint:nilerr // reason: the shared client still applies the global static route, so an unreadable pool is answered as "no plan" rather than a fault.
		return nil, nil
	}
	usable := make([]domain.Proxy, 0, len(rows))
	for _, row := range rows {
		if row.RouteUsable() {
			usable = append(usable, row)
		}
	}
	if len(usable) == 0 {
		return nil, nil
	}

	byID := make(map[string]domain.Proxy, len(usable))
	for _, row := range usable {
		byID[row.ID()] = row
	}
	ids := s.candidateOrder(ctx, providerID, binding, usable)

	attempts := make([]domain.ProxyRouteAttempt, 0, len(ids)+1)
	for _, id := range ids {
		row := byID[id]
		secret := ""
		if row.HasPassword() {
			secret, err = s.opener.Open(row.PasswordEncrypted())
			if err != nil {
				// One unreadable seal must not sink the plan: the candidate is
				// skipped, and an attempt list that empties here degrades to
				// the shared route below.
				continue
			}
		}
		endpoint, err := row.DialURL(secret)
		if err != nil {
			continue
		}
		attempts = append(attempts, domain.ProxyRouteAttempt{ID: id, URL: endpoint})
	}
	if len(attempts) == 0 {
		return nil, nil
	}

	if document.Network.OutboundProxyEnabled {
		if raw := strings.TrimSpace(document.Network.OutboundProxyURL); raw != "" {
			parsed, err := url.Parse(raw)
			if err != nil || parsed.Host == "" {
				// The static route refuses a URL it cannot dial rather than dialing
				// direct; dropping the tail here would quietly weaken that, so a
				// malformed stored value refuses the plan like any broken setting.
				return nil, fmt.Errorf("the stored outbound proxy url %q is not a usable proxy URL", raw)
			}
			attempts = append(attempts, domain.ProxyRouteAttempt{ID: "", URL: parsed})
		}
	}
	return attempts, nil
}

// ReportFailure takes one candidate out of rotation for the wait the failure
// stated (D6). It is best effort: a store that cannot park leaves the candidate in
// rotation, which is the same degradation the plan applies to its own store reads.
func (s *ProxyRouteService) ReportFailure(ctx context.Context, proxyID string, ttl time.Duration) {
	if proxyID == "" {
		return
	}
	_ = s.routes.Park(ctx, proxyID, ttl)
}
