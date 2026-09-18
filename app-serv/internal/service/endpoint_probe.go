// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_probe.go
// @for       The outbound port a connectivity test uses, expressed without any
//
//	HTTP type.
//
// @uses      internal/domain, context, time.
// @reason    SPEC-API-001 §7.5 and §7.4 both offer a connectivity test, and a
//
//	test has to reach the upstream — but AGENTS.md §1.5 forbids net/http
//	in this layer and the layer above already owns the request. Declaring
//	the port here is what lets the service orchestrate a probe (record
//	the outcome on the aggregate, trip the key's circuit) while the
//	net/http adapter lives in the composition root, so neither layer
//	learns about the other's world.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// connectivityProbeTimeout bounds one connectivity test end to end. A test
// exists to answer "does this credential work right now", so it fails fast
// rather than waiting out a chat completion's budget: SPEC-API-001 §4 gives a
// normal upstream call 120s, and a probe that took that long would be useless to
// an operator staring at a button. It stays well above the liveness budget
// (dependencyProbeTimeout) because a real upstream round trip is slower than a
// database ping.
const connectivityProbeTimeout = 15 * time.Second

// ProbeOutcome is one connectivity attempt's result, expressed in domain terms
// so the service layer never sees an HTTP type (AGENTS.md §1.5).
type ProbeOutcome struct {
	// State is domain.EndpointTestOK or domain.EndpointTestFail, the vocabulary
	// the aggregate stores in test_status.
	State string

	// LatencyMS is the round trip in milliseconds, reported even for a failure
	// because a slow rejection is a different problem from an instant one.
	LatencyMS int

	// Status is the upstream HTTP status, or 0 when the call never completed.
	// It is carried so a failure message can name what the upstream said.
	Status int

	// Message is a short English explanation safe to render in the panel, or
	// empty on success. It never contains credential material.
	Message string
}

// EndpointProber probes one configured upstream endpoint.
//
// The credential arrives already opened: this port is a transport helper, not a
// second service, so it holds no repository, no sealer, and no key store. The
// service unseals the key it chose and hands over the plaintext for the
// duration of one call.
type EndpointProber interface {
	// ProbeEndpoint calls the endpoint's provider with the given key's
	// credential and reports whether the credential is accepted. An error is
	// reserved for a failure of the probe itself (an unknown provider, an
	// unreachable registry entry); an upstream rejection is a ProbeOutcome with
	// State = fail, because "the upstream said no" is an answer, not a fault.
	ProbeEndpoint(ctx context.Context, endpoint domain.UpstreamEndpoint, key domain.UpstreamKey, credential string) (ProbeOutcome, error)
}

// NodeProber probes a custom provider node's base URL.
//
// A node carries no credential of its own (SPEC-API-001 §7.4), so the caller
// supplies one when it has one; an empty credential is a legitimate probe of a node
// whose upstream needs none.
type NodeProber interface {
	// ProbeNode calls the node's base URL and reports whether it answers.
	ProbeNode(ctx context.Context, node domain.ProviderNode, credential string) (ProbeOutcome, error)
}
