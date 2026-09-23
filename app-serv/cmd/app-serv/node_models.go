// Command app-serv adapts a provider node's model list to HTTP.
//
// @file      cmd/app-serv/node_models.go
// @for       The net/http implementation of service.NodeModelSource: read a
//
//	compatible node's own model list, and fall back when it cannot answer.
//
// @uses      internal/netguard, internal/provider, internal/registry,
//
//	internal/service, context, fmt, io, net/http, strings, sync, time.
//
// @reason    SPEC-API-001 §7.4 serves a node's models and draft 017 §4.2 measured
//
//	that a synthesized node carried none, so the node appeared in four
//	surfaces with `len(entry.Models) = 0`. AGENTS.md §1.5 forbids net/http in
//	the service layer, so the read is an adapter here and a port there, the
//	same split the connectivity probe uses.
//
//	Two rules are structural rather than incidental. The destination is
//	operator-supplied, so it goes through the same egress guard the probe
//	uses, pre-flight and at connect time (OWASP A01, draft §4.8). And the
//	answer is cached, because the overlay that calls this rebuilds on every
//	provider lookup: without a cache one panel request would dial the
//	upstream once per catalog row. The cache decisions live in
//	node_models_cache.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/netguard"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// nodeModelBodyLimit bounds the model list a node's upstream may answer with.
// A list is a few kilobytes; the limit exists so a hostile or broken upstream
// cannot make the gateway allocate without bound (OWASP A05).
const nodeModelBodyLimit = 1 << 20

// nodeModelSource reads a compatible node's model list from its upstream and
// remembers the answer for a short window.
type nodeModelSource struct {
	lookup      func(id string) (nodeTarget, bool)
	connectors  *provider.Connectors
	guard       *netguard.Guard
	credentials nodeCredentialSource
	client      *http.Client
	successTTL  time.Duration
	clock       func() time.Time

	// mu guards cache, and is held across a fetch as well as a read: that makes
	// the cache single-flight per node for free, so two concurrent lookups make
	// one request. Holding a lock across network I/O is usually wrong; here the
	// critical section is one bounded request to one host, and the alternative —
	// a second lock, a map of in-flight calls, and a wait — is more machinery
	// than a per-node read justifies.
	mu    sync.Mutex
	cache map[string]cachedNodeModels
}

// newNodeModelSource builds the adapter. The guard is a parameter, not an
// ambient dependency, because a dialer built without one is the OWASP A01 hole
// draft 017 §4.8 names — and cmd/app-serv/egress_guard_assert_test.go fails when
// an adapter is constructed without it.
func newNodeModelSource(
	lookup func(id string) (nodeTarget, bool),
	connectors *provider.Connectors,
	guard *netguard.Guard,
	credentials nodeCredentialSource,
	successTTL time.Duration,
) *nodeModelSource {
	client := dataplane.NewHTTPClient(dataplane.HTTPClientDeps{
		Dialer: guard.NewDialer(probeConnectTimeout, 0),
	})
	client.Timeout = probeConnectTimeout + 5*time.Second
	return &nodeModelSource{
		lookup:      lookup,
		connectors:  connectors,
		guard:       guard,
		credentials: credentials,
		client:      client,
		successTTL:  successTTL,
		clock:       time.Now,
		cache:       make(map[string]cachedNodeModels),
	}
}

// ListNodeModels implements service.NodeModelSource.
//
// Every failure is an answer with ModelSourceRegistry and a warning, never an
// error: a node whose upstream is down still routes, so failing this read would
// take a working node out of the panel over a list it does not need.
func (s *nodeModelSource) ListNodeModels(ctx context.Context, nodeID string) (service.NodeModelList, error) {
	if cached, ok := s.cached(nodeID); ok {
		return cached, nil
	}
	target, ok := s.lookup(nodeID)
	if !ok {
		return s.rememberFailure(nodeID, "this provider node is no longer configured"), nil
	}
	credential, err := s.credentials(ctx, nodeID)
	if err != nil {
		//nolint:nilerr // reason: an unreadable credential is answered as a fallback list rather than as a fault. A node still routes without a readable key, so a 500 would take a working node out of the panel over a list it does not need.
		return s.rememberFailure(nodeID, "the node's credential could not be read"), nil
	}
	return s.fetch(ctx, nodeID, target, credential)
}

// fetch performs one guarded read of the node's models path.
func (s *nodeModelSource) fetch(ctx context.Context, nodeID string, target nodeTarget, credential string) (service.NodeModelList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nodeModelsURL(target), nil)
	if err != nil {
		return s.rememberFailure(nodeID, service.UpstreamUnavailableWarning), nil
	}
	// Validate before sending, so a refused destination is reported as a refusal
	// rather than as an unreachable host; the dialer's Control hook repeats the
	// check on the address it actually reaches (OWASP A01).
	if err := s.guard.CheckHost(ctx, req.URL.Hostname()); err != nil {
		return s.rememberFailure(nodeID, refusalMessage("upstream", err)), nil
	}
	if err := s.applyAuth(req, target, credential); err != nil {
		return s.rememberFailure(nodeID, service.UpstreamUnavailableWarning), nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return s.rememberFailure(nodeID, service.UpstreamUnavailableWarning), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.rememberFailure(nodeID, fmt.Sprintf("the upstream answered %d", resp.StatusCode)), nil
	}
	models, err := decodeNodeModels(io.LimitReader(resp.Body, nodeModelBodyLimit))
	if err != nil || len(models) == 0 {
		return s.rememberFailure(nodeID, service.UpstreamUnavailableWarning), nil
	}
	return s.remember(nodeID, service.NodeModelList{
		Models: models,
		Source: service.ModelSourceUpstream,
	}, s.successTTL), nil
}

// applyAuth places the credential the way the node's format expects.
//
// It goes through the same connector seam the probe and the data plane use, so a
// node's read cannot place a credential differently from the request that
// follows it. An empty credential sends nothing, which is what an upstream
// needing no key requires.
func (s *nodeModelSource) applyAuth(req *http.Request, target nodeTarget, credential string) error {
	if credential == "" {
		return nil
	}
	entry := registry.Provider{
		ID:        req.URL.Hostname(),
		Category:  "apikey",
		AuthType:  registry.AuthAPIKey,
		Transport: registry.Transport{Format: target.Format, BaseURL: target.BaseURL},
	}
	return s.connectors.For(entry).ApplyAuth(req, provider.StaticKey("", "", credential))
}

// nodeModelsURL is the models path for a node's format.
//
// A node stores a base, not an endpoint URL, and the domain already stripped the
// path the transport appends (domain.normalizeNodeBaseURL), so this appends the
// one path a models read needs. Both compatible formats answer `/models`; the
// reference strips `/messages` from an Anthropic base before appending, which
// the domain boundary has already done by the time a target reaches here.
func nodeModelsURL(target nodeTarget) string {
	return strings.TrimSuffix(target.BaseURL, "/") + "/models"
}
