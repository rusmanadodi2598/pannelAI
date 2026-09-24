// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_kind.go
// @for       The kind a caller needs a model to have, and the rule that decides
//
//	whether one model satisfies it.
//
// @uses      internal/registry, strings.
// @reason    A registry entry may declare models whose payload is not a chat
//
//	request (the reference's `kind: "systemone"` decision models), and each
//	plane must refuse the other's models rather than serve them with the
//	wrong body. That is one rule read twice, so it lives here instead of
//	inside the resolution walk: the chat plane asks for a chat model, the
//	decision route asks for a decision model, and neither can drift from
//	the other's idea of which kinds are which.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// KindChat and KindSystemOne are the two planes a declared model can serve. The
// set is closed because each member names a payload vocabulary: a chat request, or
// the provider's own decision payload.
const (
	// KindChat is the chat data plane: the OpenAI, Anthropic, and Responses
	// wires the gateway translates.
	KindChat = "chat"
	// KindSystemOne is the native decision route (SPEC-API-001 §7.15). It is the
	// reference's own spelling of the kind, so the registry and this constant
	// cannot disagree.
	KindSystemOne = "systemone"
)

// modelServesKind reports whether a declared model answers a plane's payload.
//
// A chat plane accepts a model whose kind is unset, `llm`, or `chat`, which are
// the three spellings the document uses for an ordinary model; every other kind
// is a different vocabulary. The decision plane accepts only its own kind, which
// is the conservative direction for it: a model that does not declare the kind is
// a chat model, and serving it on the decision route would send a payload the
// upstream reads as something else.
func modelServesKind(model registry.Model, kind string) bool {
	declared := strings.ToLower(strings.TrimSpace(model.Kind))
	if kind == KindSystemOne {
		return declared == KindSystemOne
	}
	switch declared {
	case "", "llm", "chat":
		return true
	default:
		return false
	}
}

// kindRefusal explains why a model cannot serve a plane, naming the kind so the
// reason is readable rather than a bare "not found".
//
// The two planes answer with different codes on purpose, and each matches what
// the code means to a caller. A model of another kind asked for on the chat plane
// is a model the chat plane does not have, which is MODEL_NOT_FOUND: the catalog
// list already omits it, so a client asking for it named something absent. The
// decision route instead refuses by name what it does have but cannot serve,
// which is PROVIDER_NOT_ROUTABLE: the route exists, the model exists, and the
// combination is what is unsupported.
func kindRefusal(providerID, modelID, declared, wanted string) error {
	if wanted == KindSystemOne {
		return dataPlaneError(CodeProviderNotRoutable,
			"model "+providerID+"/"+modelID+" is not a decision model (kind "+describeKind(declared)+")")
	}
	return dataPlaneError(CodeModelNotFound,
		"model "+providerID+"/"+modelID+" is not a chat model (kind "+describeKind(declared)+")")
}

// describeKind names a kind for an error message, spelling an unset one rather
// than leaving a gap in the sentence.
func describeKind(kind string) string {
	if strings.TrimSpace(kind) == "" {
		return "unset"
	}
	return kind
}

// resolveForKind is the walk itself, parameterised by the plane the caller
// serves, so the chat and decision routes cannot disagree about which models
// they accept.
func (r *Resolver) resolveForKind(ctx context.Context, model, kind string) (Resolution, error) {
	return r.resolveWithinKind(ctx, model, nil, 0, kind)
}
