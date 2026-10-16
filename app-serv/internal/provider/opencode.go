// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode.go
// @for       The OpenCode Free connector: the client identity it presents and the session it names.
// @uses      internal/registry, net/http, strings.
// @reason    OpenCode Free is a no-auth provider whose models answer on three different paths and whose free tier refuses a request that does not carry the CLI's own identity headers. Both are upstream rules, so both belong to a connector rather than to a branch on the provider id in shared code, which is the property the plugin seam exists to give.
//
//	The request shape lives in opencode_body.go, the session identity in
//	opencode_session.go, and the per-model URL in opencode_url.go, for the
//	AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-21
package provider

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// OpenCodeUserAgent is the client identity the free tier checks. It names the
// v2 CLI, because the v1 `opencode/<major>.<minor>` form is refused with 403
// FreeTierError since 2026-10-14 (the reference's own OPENCODE_UA).
const OpenCodeUserAgent = "opencode/latest/2.0.27/cli"

// openCodeUserAgentRE is the v2 identity shape: opencode/<channel>/<semver>/
// <client>. The match is anchored so a browser string carrying an incidental
// `opencode/...` token is not mistaken for the CLI's own agent.
var openCodeUserAgentRE = regexp.MustCompile(`(?i)^opencode/[a-z0-9-]+/(\d+)\.\d+\.\d+/[a-z0-9-]+$`)

// openCodeValidUserAgent reports whether a downstream User-Agent is one the
// upstream accepts as the official client's own: the v2 shape with a major at
// or above 2. A v1-style, bare, or absent agent is replaced with
// OpenCodeUserAgent before the request leaves.
func openCodeValidUserAgent(userAgent string) bool {
	match := openCodeUserAgentRE.FindStringSubmatch(strings.TrimSpace(userAgent))
	if match == nil {
		return false
	}
	major, err := strconv.Atoi(match[1])
	return err == nil && major >= 2
}

// The identity headers the free tier reads, and the value the CLI sends for the
// project. They are constants because the upstream compares them literally. The
// v2 CLI presents the session under three more names beside x-opencode-session,
// and no longer sends the v1 turn header at all.
const (
	openCodeClientHeader          = "x-opencode-client"
	openCodeSessionHeader         = "x-opencode-session"
	openCodeSessionIDHeader       = "x-opencode-session-id"
	openCodeSessionAffinityHeader = "x-session-affinity"
	openCodeSessionShortHeader    = "x-session-id"
	openCodeProjectHeader         = "x-opencode-project"
	openCodeProjectValue          = "global"
	openCodeClientDefault         = "cli"
)

// OpenCodeAnthropicVersion is the API version the Anthropic Messages wire requires
// on every request. It is also the value the registry declares for the anthropic
// entry, so no new value is invented here.
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

// clientValue keeps a client identity the registry declares, so an entry naming
// a different editor is honoured rather than overwritten by the default. The
// request's own header is read first because the core copies the entry's
// declared headers onto it before this runs.
func (c *OpenCode) clientValue(req *http.Request) string {
	if declared := strings.TrimSpace(req.Header.Get(openCodeClientHeader)); declared != "" {
		return declared
	}
	return c.clientValueFor(c.entry)
}

// clientValueFor answers the client identity an entry declares, falling back to
// the CLI's own, so the session ApplyAuth presents and the session SessionKey
// names cannot drift apart.
func (c *OpenCode) clientValueFor(entry registry.Provider) string {
	if declared := strings.TrimSpace(entry.Transport.Headers[openCodeClientHeader]); declared != "" {
		return declared
	}
	if declared := strings.TrimSpace(c.entry.Transport.Headers[openCodeClientHeader]); declared != "" {
		return declared
	}
	return openCodeClientDefault
}

// SessionKey implements provider.SessionSerializer: it names the session one
// call presents, so the transport keeps a single request in flight per session.
// The value is minted from the same pair ApplyAuth derives the identity headers
// from, the entry's client identity and the endpoint's id, so the key and the
// header agree by construction.
//
// A keyed lane presents no session at all: ApplyAuth places the credential and
// returns before the identity headers, so the upstream accounts that traffic by
// key rather than by session. Naming a session there would serialize a lane that
// has no session to protect, so the key is empty exactly when the headers are.
func (c *OpenCode) SessionKey(req Request) string {
	entry := c.entry
	if !entry.NoAuth && entry.AuthType != registry.AuthNone {
		if _, value, err := req.Credential.Secret(); err == nil && value != "" {
			return ""
		}
	}
	return OpenCodeSessionFor(c.clientValueFor(c.entryFor(req)), req.Credential.EndpointID())
}

// ForcesStream reports that this provider only answers a streamed request. The
// free tier refuses a non-streaming one with 403, so the core has to fold the
// stream back for a client that asked for a single body.
func (c *OpenCode) ForcesStream() bool { return true }
