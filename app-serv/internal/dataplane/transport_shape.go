// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_shape.go
// @for       The optional connector seams the transport applies before a call:
//
//	a request transform, and a declared forced stream.
//
// @uses      internal/provider, internal/registry, encoding/json.
// @reason    A provider may need its outbound request rewritten, or may refuse a
//
//	non-streaming one, and neither is something the core should learn by
//	provider id. Both are optional interfaces a connector implements, so
//	they are applied here through one type assertion each and a connector
//	that implements neither is untouched. They live apart from the call
//	loop so that file stays about policy, as AGENTS.md §1.1 asks.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-21
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// applyShape lets the connector rewrite the outbound request when it implements
// the optional Transformer seam, which is how a provider that only accepts a
// particular shape is served without the core branching on its id.
//
// It runs before the URL is built, because a connector may need the shaped
// request to decide the URL, and its failure is a client error: the body it
// could not read came from the client's own request.
func applyShape(plugin provider.Plugin, request *provider.Request) error {
	transformer, ok := plugin.(provider.Transformer)
	if !ok {
		return nil
	}
	if err := transformer.TransformRequest(request); err != nil {
		return wrapDataPlaneError(CodeValidation, "the request could not be shaped for this provider", err)
	}
	return nil
}

// forcesStream reports whether a connector declares that its provider refuses a
// non-streaming request.
func forcesStream(plugin provider.Plugin) bool {
	forcer, ok := plugin.(provider.StreamForcer)
	return ok && forcer.ForcesStream()
}

// forceStreamMember sets the `stream` member of an outbound body, which is the
// mechanical consequence of a connector declaring that its provider only answers
// a stream. It reports a body it cannot read as a client error rather than
// forwarding one it has not corrected.
func forceStreamMember(body []byte) ([]byte, error) {
	decoded, ok := decodeObject(body)
	if !ok {
		return nil, dataPlaneError(CodeValidation, "the request body could not be read")
	}
	decoded["stream"] = mustJSON(true)
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, wrapDataPlaneError(CodeValidation, "the request body could not be re-encoded", err)
	}
	return encoded, nil
}

// ForcesStream reports whether the connector for a provider refuses a
// non-streaming request, so a caller knows the answer will arrive as a stream
// even when it asked for one body.
func (t *Transport) ForcesStream(entry registry.Provider) bool {
	return forcesStream(t.connectors.For(entry))
}
