// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_transports.go
// @for       The multi-endpoint rule: which endpoint a model is served on, and
//
//	which credential placement that endpoint reads.
//
// @uses      fmt, strings, internal/registry.
// @reason    opencode-go and opencode-zen declare three endpoints each
//
//	(`transports[]` in the reference), and a model's `supportedFormats`
//	decides which of them may answer it. Without this rule the connector
//	built a path by appending a leaf to the entry's chat URL, which
//	produced ".../chat/completions/zen/v1/messages": a URL that answers
//	404 and names no cause. The endpoint table also carries the credential
//	placement per wire, because the Messages endpoint reads a raw
//	`x-api-key` where the chat endpoint reads a bearer.
//
//	It lives apart from opencode.go because the resolution is a table
//	lookup with its own failure modes, and AGENTS.md §1.1 asks for the
//	split before the limit forces it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package provider

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// openCodeEndpointFor picks the endpoint a resolved model is served on.
//
// A single-endpoint provider (no `transports`) answers on its declared base URL
// and needs no choice, so it reports not-found and the caller keeps the URL rule
// it already had.
//
// A multi-endpoint provider chooses by the wire the request is being translated
// into: the entry's own endpoint table names one URL per wire, and the model's
// declared formats guard the choice. A model that lists `supportedFormats` and
// does not include that wire is answered by the provider's own default URL, which
// is the reference's own rule: an unmatched format leaves `useTransport` null, so
// the request keeps the provider's `baseUrl` (chatCore.js:95-101, and
// executors/default.js:106-109 uses `config.baseUrl` when no runtime transport was
// attached).
//
// Refusing instead was tried and is wrong: it removed 43 of opencode-zen's models
// from routing entirely (every claude-only id, because the provider's default wire
// is openai), and the reference serves those models whenever the client speaks
// their wire.
func openCodeEndpointFor(entry registry.Provider, model registry.Model, wire string) (registry.TransportEndpoint, bool) {
	if len(entry.Transports) == 0 {
		return registry.TransportEndpoint{}, false
	}
	for _, endpoint := range entry.Transports {
		if strings.TrimSpace(endpoint.Format) != wire {
			continue
		}
		if !openCodeModelSupportsFormat(model, wire) {
			// The model declared its own formats and this wire is not one of
			// them, so the endpoint table cannot serve the request and the
			// provider's default URL answers, exactly as the reference does.
			return registry.TransportEndpoint{}, false
		}
		return endpoint, true
	}
	return registry.TransportEndpoint{}, false
}

// openCodeModelSupportsFormat reports whether a model may be served on a wire.
//
// An empty `supportedFormats` means the model declares nothing, which the
// reference reads as "every transport is allowed": the guard only ever narrows
// a model that stated its own formats.
func openCodeModelSupportsFormat(model registry.Model, wire string) bool {
	if len(model.SupportedFormats) == 0 {
		return true
	}
	for _, format := range model.SupportedFormats {
		if strings.TrimSpace(format) == wire {
			return true
		}
	}
	return false
}
