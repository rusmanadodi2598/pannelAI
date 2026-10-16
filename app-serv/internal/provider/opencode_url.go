// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_url.go
// @for       Which URL a resolved OpenCode model answers on, and the wire that decides it.
// @uses      fmt, strings, internal/registry.
// @reason    The free tier's models answer on three different paths while the provider entry declares only one format, so a `muse-spark-*` model would be sent to the chat URL that cannot parse its body. Operators store the base in any of five shapes (bare host, inference base, the legacy Zen base, a full path, a trailing slash), and composing them into one correct URL is an upstream rule rather than a branch on the provider id in shared code.
//
//	It lives apart from opencode.go for the AGENTS.md §1.1 budget, and
//	beside opencode_transports.go, which answers the endpoint a model is
//	served on rather than the path under it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"fmt"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// openCodeInferencePrefix is the path segment the free tier lives under since
// OpenCode v2. A base that already carries it is joined with the leaf alone, so
// an operator who stored the documented `https://opencode.ai/inference/v1` base
// does not get a doubled path.
const openCodeInferencePrefix = "/inference/v1"

// openCodeLegacyZenPrefix is the v1 path the free tier answered on before the
// move; /zen/v1 now answers every free request with 403 FreeTierError. A base
// an operator stored with it is rewritten onto the new prefix rather than
// composed into a path the upstream no longer serves. It never matches the
// subscription lane's /zen/go/v1 base.
const openCodeLegacyZenPrefix = "/zen/v1"

// The leaves each wire answers on. The Responses API and the Anthropic Messages
// wire have their own paths; everything else is chat completions.
const (
	openCodeResponsesLeaf = "/responses"
	openCodeMessagesLeaf  = "/messages"
	openCodeChatLeaf      = "/chat/completions"
)

// openCodeClaudeWire is the Anthropic wire's name, which the registry spells
// without exporting a constant because only its own document uses it.
const openCodeClaudeWire = "claude"

// Endpoint builds the URL the resolved model answers on. A multi-endpoint provider
// answers on the endpoint its model's own wire names, and that URL is used as
// written because it is already complete. A single-endpoint provider composes the
// URL from its base, with the model's wire deciding the path: the provider's
// declared format cannot express it, because a `muse-spark-*` model answers on the
// Responses API while the provider itself speaks chat completions. The base is
// read in every shape an operator stores it in (bare host, inference base, the
// legacy Zen base, full path, trailing slash) and composes into one correct URL.
func (c *OpenCode) Endpoint(req Request, _ Credential) (string, error) {
	entry := c.entryFor(req)
	wire := opencodeWire(req, entry)

	if endpoint, found := openCodeEndpointFor(entry, req.Model, wire); found {
		// An endpoint may declare no URL of its own, carrying only a format and a
		// credential placement per wire (the reference's xiaomi-tokenplan table).
		// Such an endpoint keeps the entry's own URL; only the credential
		// placement and headers come from the table.
		base := strings.TrimSpace(endpoint.BaseURL)
		if base == "" {
			return openCodeDefaultURL(entry, wire)
		}
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			return "", fmt.Errorf("provider %s: the %s endpoint is not absolute", entry.ID, wire)
		}
		return strings.TrimRight(base, "/") + strings.TrimSpace(endpoint.URLSuffix), nil
	}

	return openCodeDefaultURL(entry, wire)
}

// openCodeDefaultURL builds the URL from the entry's own base, which answers a
// single-endpoint provider and a multi-endpoint request the table did not match. A
// multi-endpoint entry stores a complete URL in base_url, so it is used as written:
// appending a leaf would build a path nothing serves. A single-endpoint entry may
// store either a complete URL or a base, so the leaf is composed only when the base
// does not already name it, and a base still carrying the v1 /zen/v1 prefix is
// rewritten onto the /inference/v1 prefix the free tier moved to.
func openCodeDefaultURL(entry registry.Provider, wire string) (string, error) {
	base := strings.TrimSpace(entry.Transport.BaseURL)
	if base == "" && len(entry.Transport.BaseURLs) > 0 {
		base = strings.TrimSpace(entry.Transport.BaseURLs[0])
	}
	if base == "" {
		return "", fmt.Errorf("provider %s: no base_url is configured", entry.ID)
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return "", fmt.Errorf("provider %s: base_url is not absolute", entry.ID)
	}
	base = strings.TrimRight(base, "/")

	if len(entry.Transports) > 0 {
		return base, nil
	}

	base = strings.Replace(base, openCodeLegacyZenPrefix, openCodeInferencePrefix, 1)
	leaf := openCodeLeaf(wire)
	if strings.HasSuffix(base, leaf) {
		// The base already names the endpoint, so it is used as written.
		return base, nil
	}
	if strings.HasSuffix(base, openCodeInferencePrefix) || strings.Contains(base, openCodeInferencePrefix+"/") {
		// The base already carries the prefix, so only the leaf is missing.
		return base + leaf, nil
	}
	return base + openCodeInferencePrefix + leaf, nil
}

// opencodeWire reports the wire the request is being sent in. The translation's own
// answer wins when present, because that is the wire the body actually is: a model
// may declare one target format while the request was translated into another, and
// a table keyed on the wrong value sends the body to a URL that cannot parse it.
// The model's declared target is the fallback for a caller that resolved a model
// without translating (the media and decision planes build their own bodies), and
// the provider's own format is the last resort.
func opencodeWire(req Request, entry registry.Provider) string {
	if wire := strings.TrimSpace(req.Wire); wire != "" {
		return wire
	}
	if target := strings.TrimSpace(req.Model.TargetFormat); target != "" {
		return target
	}
	if format := strings.TrimSpace(entry.Transport.Format); format != "" {
		return format
	}
	return strings.TrimSpace(req.Provider.Transport.Format)
}

// openCodeLeaf maps a wire onto the path leaf it answers on.
func openCodeLeaf(wire string) string {
	switch wire {
	case registry.FormatOpenAIResponses:
		return openCodeResponsesLeaf
	case openCodeClaudeWire:
		return openCodeMessagesLeaf
	default:
		return openCodeChatLeaf
	}
}

// isOpenCodeMessagesURL reports whether a path names the Messages leaf. The
// comparison includes the separating slash, so a nested prefix matches while a
// path that merely ends in the same letters ("notmessages") does not.
func isOpenCodeMessagesURL(path string) bool {
	return strings.HasSuffix(strings.TrimRight(path, "/"), openCodeMessagesLeaf)
}
