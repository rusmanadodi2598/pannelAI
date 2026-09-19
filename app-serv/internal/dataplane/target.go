// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/target.go
// @for       The upstream wire formats the gateway can translate, and the mapping
//
//	from a registry format onto one.
//
// @uses      internal/registry.
// @reason    SPEC-API-001 §8 makes PROVIDER_NOT_ROUTABLE the answer for a format
//
//	with no translator, so "which formats have one" is one decision that
//	both the resolver and its tests read. It lives in its own file so
//	resolve.go stays inside the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
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
)

// targetFormat maps a registry wire format onto a translator, or "" when no
// translator handles it.
//
// Gemini is deliberately absent: the registry reports every provider declaring it
// as `routability: connector` (they wrap the payload in a vendor envelope), so a
// format with no reachable route must not decode as translatable. The Gemini
// payload builder exists for the connector that will need it, and is exercised
// directly by its own tests.
func targetFormat(format string) string {
	switch format {
	case registry.DefaultFormat:
		return TargetOpenAI
	case TargetClaude:
		return TargetClaude
	case registry.FormatOpenAIResponses:
		return TargetResponses
	default:
		return ""
	}
}
