// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/node_models.go
// @for       The port a custom provider node's model list is read through, and
//
//	the value it answers with.
//
// @uses      internal/domain, internal/registry, context.
// @reason    SPEC-API-001 §7.4 serves a node's models, and a compatible node's
//
//	models live at its own upstream rather than in the embedded document
//	(draft 017 §4.2). AGENTS.md §1.5 forbids net/http in this layer, so the
//	read is a port here and the adapter lives in the composition root —
//	the same split EndpointProber and NodeProber already use.
//
//	The origin travels with the list because the two answers are not
//	interchangeable: a list read from the upstream is what the node offers
//	right now, while a fallback is what was known before. A client that
//	could not tell them apart would present a stale list as current, which
//	is the failure the `source` field exists to prevent.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Model-list origins. The two values are part of the wire (§7.4 `source`), so
// they are named once here rather than spelled at each return.
const (
	// ModelSourceUpstream means the list was read from the node's own upstream
	// during this request's window.
	ModelSourceUpstream = "upstream"
	// ModelSourceRegistry means the list came from what the registry already
	// held: the node's declared models, or a stored list from an earlier read.
	ModelSourceRegistry = "registry"
)

// UpstreamUnavailableWarning is the message a client sees when the upstream's
// answer could not be used. It is a fixed English sentence with no upstream text
// in it: an upstream body can echo the credential back, and OWASP A09 forbids
// that reaching a response or a log.
//
// It is exported because the adapter that performs the read also produces it for
// the failures it detects before the service sees an answer, and two spellings of
// one sentence is how a client ends up parsing a message that changed.
const UpstreamUnavailableWarning = "the upstream model list could not be read"

// NodeModelList is one node's model list and where it came from.
type NodeModelList struct {
	// Models is the list, in the upstream's own order.
	Models []registry.Model
	// Source is ModelSourceUpstream or ModelSourceRegistry.
	Source string
	// Warning explains, in English, why Source is not upstream. It is empty
	// when the upstream answered.
	Warning string
}

// NodeModelSource reads one custom provider node's model list from its upstream.
//
// The node is named by id rather than handed over as a domain value because the
// two callers hold different things: the overlay has the stored node, while the
// provider read has only the synthesized entry the index hands out. Naming the
// id lets one port serve both, and keeps the adapter — which is the only layer
// that may read storage and dial — free to load what it needs.
//
// An implementation returns ModelSourceRegistry with a Warning rather than an
// error when the upstream cannot answer: a node whose upstream is down must
// still resolve, or a model-list problem becomes a routing outage.
type NodeModelSource interface {
	ListNodeModels(ctx context.Context, nodeID string) (NodeModelList, error)
}
