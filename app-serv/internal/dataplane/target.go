// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/target.go
// @for       The upstream wire formats the gateway can translate, and the mapping from a registry format onto one.
// @uses      internal/registry.
// @reason    SPEC-API-001 §8 makes PROVIDER_NOT_ROUTABLE the answer for a format with no translator, so "which formats have one" is one decision that both the resolver and its tests read. It lives in its own file so resolve.go stays inside the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"

// TargetFormat is the upstream wire format the gateway can translate. Only the
// formats with a translator are listed: a provider declaring anything else is
// refused as PROVIDER_NOT_ROUTABLE rather than sent as a 502 that reads like an
// upstream outage.
const (
	TargetOpenAI = "openai"
	TargetClaude = "claude"
	// TargetResponses is the OpenAI Responses API, which P3 added a translator
	// for: a custom node with `api_type: responses` is routable from either
	// client wire.
	TargetResponses = "openai-responses"
	// TargetGemini is Gemini's generateContent wire: the model in the path, the
	// payload written plainly.
	TargetGemini = "gemini"
)

// targetFormat maps a registry wire format onto a translator, or "" when no
// translator handles it.
//
// Three Google formats stay unmapped on purpose, and each for its own reason.
// `gemini-cli` and `antigravity` wrap the same Gemini payload in a Cloud Code
// Assist envelope keyed to a project (measured against the reference's executor:
// the URL has no model segment either), so a translator alone would send a body
// the vendor rejects; `vertex` addresses calls by project and region in the path.
// Each needs its own connector before it can be served, and a format with no
// reachable route must not decode as translatable.
func targetFormat(format string) string {
	switch format {
	case registry.DefaultFormat:
		return TargetOpenAI
	case TargetClaude:
		return TargetClaude
	case registry.FormatOpenAIResponses:
		return TargetResponses
	case TargetGemini:
		return TargetGemini
	default:
		return ""
	}
}
