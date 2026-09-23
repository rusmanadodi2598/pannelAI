// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_validate_plan.go
// @for       The per-format plan a credential check follows: which request to
//
//	make, and which status rule decides the answer.
//
// @uses      internal/registry, strings.
// @reason    Draft 017 §4.2's second consequence is that a provider's validation
//
//	surface is a property of its FORMAT rather than of its id: the
//	reference declares 18 URLs in its registry and writes the rest into a
//	per-family switch inside its validate route. A per-format plan is what
//	lets one rule serve a family — 43 OpenAI-wire providers, 7 on the
//	Anthropic wire — instead of 94 id cases, which is the shape draft
//	§4.2 asks for ("per format, bukan per provider id").
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// ValidatePlan is how one provider's credential is checked.
type ValidatePlan struct {
	// URL is the request target, or empty when no check is possible.
	URL string
	// Method is "GET" or "POST". A provider whose base URL is already the
	// messages endpoint is probed by POSTing to it, because it serves no model
	// list to read.
	Method string
	// AnthropicRule reports whether only 401/403 mean a rejected credential. It
	// is set for the Anthropic wire, where a 400 means the *request* was wrong
	// and a 529 that the provider is overloaded (reference
	// providers/validate/route.js:178).
	AnthropicRule bool
	// NeedsModel reports whether the request body must name a model. Only a POST
	// probe has a body, and an upstream rejects an empty model name before it
	// reads the credential.
	NeedsModel bool
}

// PlanFor derives the validation plan for a registry entry.
//
// The order is the reference's, generalised to the format rather than the id:
//
//  1. A declared `transport.validate_url` wins. That is the 18 entries whose
//     registry file names its own probe.
//  2. A base URL naming a chat path yields the models path beside it
//     (`/chat/completions` or `/chatbot` → `/models`). That is 19 more.
//  3. A `claude`-format base that IS the messages endpoint is probed by POSTing
//     to it. Anthropic's wire serves no model list, which is why the reference
//     posts a one-token message and reads the status (7 providers here).
//  4. A `gemini`-format base is already its own models path, so it is read
//     directly (1 provider here).
//
// Anything else returns an empty plan, and the caller reports the reason rather
// than guessing a path: a guess produces a 404 that reads like a credential
// problem.
func PlanFor(entry registry.Provider) ValidatePlan {
	if declared := strings.TrimSpace(entry.Transport.ValidateURL); declared != "" {
		return ValidatePlan{URL: declared, Method: "GET"}
	}
	base := strings.TrimSpace(entry.Transport.BaseURL)
	if base == "" {
		return ValidatePlan{}
	}
	format := strings.ToLower(strings.TrimSpace(entry.Transport.Format))
	if derived := derivedModelsURL(base); derived != "" {
		return ValidatePlan{URL: derived, Method: "GET", AnthropicRule: format == "claude"}
	}
	if format == "claude" && strings.HasSuffix(base, "/messages") {
		return ValidatePlan{URL: base, Method: "POST", AnthropicRule: true, NeedsModel: true}
	}
	if format == "gemini" && strings.Contains(base, "/models") {
		return ValidatePlan{URL: base, Method: "GET"}
	}
	return ValidatePlan{}
}

// derivedModelsURL turns a chat-path base URL into the models path beside it,
// applying the reference's own rule (providers/validate/route.js:626-627).
// It returns "" when the base names no chat path.
func derivedModelsURL(base string) string {
	derived := strings.TrimSuffix(base, "/chat/completions")
	if derived == base {
		derived = strings.TrimSuffix(base, "/chatbot")
	}
	if derived == base {
		return ""
	}
	return derived + "/models"
}
