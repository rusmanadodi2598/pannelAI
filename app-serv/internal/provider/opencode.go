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
// The model's own wire decides the path, which is the rule the provider's
// declared format cannot express: a `muse-spark-*` model answers on the
// Responses API while the provider itself speaks chat completions, and a model
// that declares no wire falls back to the provider's own. The base is read as an
// operator would store it — a bare host, the documented Zen base, a full path,
// or a trailing slash — so every shape composes into one correct URL.
func (c *OpenCode) Endpoint(req Request, _ Credential) (string, error) {
	entry := c.entryFor(req)
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

	leaf := openCodeLeaf(opencodeWire(req, entry))
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

// opencodeWire reports the wire the model answers in: its own declared target
// format when it has one, otherwise the provider's transport format. Deriving it
// is what lets a model added to the registry tomorrow route correctly with no
// change here.
func opencodeWire(req Request, entry registry.Provider) string {
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

// ApplyAuth places the free tier's credential and identity on the request.
//
// The credential is always the literal public bearer: the free tier pools
// anonymous traffic, so a configured key would both be ignored and leak to an
// endpoint that has no use for it. The session is derived from the endpoint so
// one account presents one stable identity.
func (c *OpenCode) ApplyAuth(req *http.Request, cred Credential) error {
	req.Header.Set("Authorization", "Bearer public")
	req.Header.Set("User-Agent", OpenCodeUserAgent)
	req.Header.Set(openCodeClientHeader, c.clientValue(req))
	req.Header.Set(openCodeSessionHeader, OpenCodeSession(cred.EndpointID))
	req.Header.Set(openCodeProjectHeader, openCodeProjectValue)
	return nil
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
