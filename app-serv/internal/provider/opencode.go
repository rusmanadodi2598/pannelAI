// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode.go
// @for       The OpenCode Free connector: its per-model URL and the identity
//
//	headers the free tier gates on.
//
// @uses      internal/registry, fmt, net/http, strings.
// @reason    OpenCode Free is a no-auth provider whose models answer on three
//
//	different paths and whose free tier refuses a request that does not
//	carry the CLI's own identity headers. Both are upstream rules, so both
//	belong to a connector rather than to a branch on the provider id in
//	shared code, which is the property the plugin seam exists to give.
//	The request shape lives in opencode_body.go and the session identity
//	in opencode_session.go, for the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// OpenCodeUserAgent is the client identity the free tier checks. It has to name
// the CLI with a version at or above 1.17, because a bare or absent agent is
// refused with 403 (the reference's OPENCODE_UA, commit 6091ff59).
const OpenCodeUserAgent = "opencode/1.18.31"

// The identity headers the free tier reads, and the value the CLI sends for the
// project. They are constants because the upstream compares them literally.
const (
	openCodeClientHeader  = "x-opencode-client"
	openCodeSessionHeader = "x-opencode-session"
	openCodeProjectHeader = "x-opencode-project"
	openCodeProjectValue  = "global"
	openCodeClientDefault = "desktop"
)

// openCodeZenPrefix is the path segment the Zen API lives under. A base that
// already carries it is joined with the leaf alone, so an operator who stored
// the documented `https://opencode.ai/zen/v1` base does not get a doubled path.
const openCodeZenPrefix = "/zen/v1"

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

// OpenCodeAnthropicVersion is the API version the Anthropic Messages wire
// requires on every request. It is the reference's own constant
// (providers/shared.js:24, written at executors/opencode.js:484) and the value
// the registry declares for the anthropic entry, so no new value is invented
// here.
const OpenCodeAnthropicVersion = "2023-06-01"

// openCodeAnthropicVersionHeader is the header the Messages wire reads. It is
// lowercase because HTTP header names are case-insensitive and the wire's own
// documentation spells it that way.
const openCodeAnthropicVersionHeader = "anthropic-version"

// OpenCode is the connector for the OpenCode Free provider.
//
// It holds only the registry entry it was built for, so it carries no
// per-request state and is safe to share across concurrent requests.
type OpenCode struct {
	Base

	entry registry.Provider
}

// NewOpenCode builds the connector for one registry entry.
func NewOpenCode(entry registry.Provider) *OpenCode {
	return &OpenCode{
		Base: Base{
			ID:     entry.ID,
			Auth:   entry.AuthType,
			Format: entry.Transport.Format,
		},
		entry: entry,
	}
}

// entryFor returns the registry entry this call belongs to: the one the request
// carries when it has one, otherwise the one the connector was built for.
func (c *OpenCode) entryFor(req Request) registry.Provider {
	if strings.TrimSpace(req.Provider.Transport.BaseURL) != "" || len(req.Provider.Transport.BaseURLs) > 0 {
		return req.Provider
	}
	return c.entry
}

// Endpoint builds the URL the resolved model answers on.
//
// A multi-endpoint provider answers on the endpoint its model's own wire names
// (opencode-go and opencode-zen declare three each), and that endpoint's URL is
// used as written because it is already complete. A single-endpoint provider
// composes the URL from its base: the model's own wire decides the path, which
// is the rule the provider's declared format cannot express, because a
// `muse-spark-*` model answers on the Responses API while the provider itself
// speaks chat completions. The base is read as an operator would store it (a
// bare host, the documented Zen base, a full path, or a trailing slash), so
// every shape composes into one correct URL.
func (c *OpenCode) Endpoint(req Request, _ Credential) (string, error) {
	entry := c.entryFor(req)
	wire := opencodeWire(req, entry)

	if endpoint, found := openCodeEndpointFor(entry, req.Model, wire); found {
		// An endpoint may declare no URL of its own: the reference's
		// xiaomi-tokenplan table carries only a format and a credential placement
		// per wire, and the executor composes the URL from the entry's base. So an
		// endpoint without a base_url keeps the entry's own URL, and only the
		// credential placement and headers come from the table.
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

// openCodeDefaultURL builds the URL from the entry's own base, which is what
// answers a single-endpoint provider and a multi-endpoint request the table did
// not match.
//
// A multi-endpoint entry stores a complete URL in base_url, so that URL is used
// as written rather than composed with a leaf: appending one would build a path
// nothing serves, which is the shape a first reading of this rule produced. A
// single-endpoint entry may store either a complete URL or a base, so the leaf is
// composed only when the base does not already name it.
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

	leaf := openCodeLeaf(wire)
	if strings.HasSuffix(base, leaf) {
		// The base already names the endpoint, so it is used as written.
		return base, nil
	}
	if strings.HasSuffix(base, openCodeZenPrefix) || strings.Contains(base, openCodeZenPrefix+"/") {
		// The base already carries the Zen prefix, so only the leaf is missing.
		return base + leaf, nil
	}
	return base + openCodeZenPrefix + leaf, nil
}

// opencodeWire reports the wire the request is being sent in.
//
// The translation's own answer wins when it is present, because that is the wire
// the body actually is: a model may declare one target format while the request
// was translated into another, and an endpoint table keyed on the wrong value
// would send the body to a URL that cannot parse it. The model's declared target
// is the fallback for a caller that resolved a model without translating (the
// media and decision planes build their own bodies), and the provider's own
// format is the last resort.
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

// clientValue keeps a client identity the registry declares, so an entry naming
// a different editor is honoured rather than overwritten by the default. The
// request's own header is read first because the core copies the entry's
// declared headers onto it before this runs.
func (c *OpenCode) clientValue(req *http.Request) string {
	if declared := strings.TrimSpace(req.Header.Get(openCodeClientHeader)); declared != "" {
		return declared
	}
	if declared := strings.TrimSpace(c.entry.Transport.Headers[openCodeClientHeader]); declared != "" {
		return declared
	}
	return openCodeClientDefault
}

// ForcesStream reports that this provider only answers a streamed request. The
// free tier refuses a non-streaming one with 403, so the core has to fold the
// stream back for a client that asked for a single body.
func (c *OpenCode) ForcesStream() bool { return true }
