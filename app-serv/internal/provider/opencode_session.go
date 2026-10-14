// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session.go
// @for       The canonical OpenCode session identity a free-tier request carries.
// @uses      crypto/sha256, strings, time, internal/provider/opencode_session_cache.go.
// @reason    The free tier accounts quota per session and refuses any value that is not the CLI's own canonical shape, so a gateway that mints a random id per request burns the quota it is trying to spend (the reference fixed exactly that: one stable session per identity, commit 0c6ab4f9). Since OpenCode 1.19.0 the twelve hex characters are read back as a timestamp, which is the current clock at mint time, so no pure function of the endpoint id can produce a value the upstream accepts: one identity now has to remember what it minted.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-09-21
package provider

import (
	"crypto/sha256"
	"regexp"
	"strings"
	"time"
)

// OpenCodeSessionRE is the session shape the upstream accepts:
// `ses_` followed by twelve lowercase hex digits and fourteen base62
// characters (the reference's own OPENCODE_SESSION_RE). Any other
// value is refused with 403, so the connector mints only this shape.
var OpenCodeSessionRE = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)

// base62Alphabet is the reference's alphabet, in its order.
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// openCodeSessionCachePrefix is the reference's namespace for translated
// sessions, kept distinct from the request-id store below.
const openCodeSessionCachePrefix = "opencode:tr:"

// openCodeGenericClient is what the reference substitutes when no client tool
// names itself, so a session minted for an anonymous client is a different
// identity from one minted for a named one.
const openCodeGenericClient = "generic"

// openCodeSessions is the remembered session per (client tool, endpoint).
var openCodeSessions = newOpenCodeIDCache()

// OpenCodeSession derives the canonical session id one endpoint presents. It is
// stable per identity: the same endpoint presents the same session across every
// request, so its quota is not spent on fresh sessions, and two endpoints hash to
// different ids, so one caller's quota is never spent by another's traffic.
//
// The layout mirrors the reference's `translateSessionId`. The twelve hex
// characters carry the mint-time clock descending, because the upstream decodes
// them, and the fourteen base62 characters carry the identity, which is what makes
// one endpoint's session its own and never another's.
func OpenCodeSession(endpointID string) string {
	return openCodeSessionID(openCodeClientTool(""), endpointID)
}

// OpenCodeSessionFor presents the session belonging to one client tool on one
// endpoint, which is the pair the reference keys its own store by.
func OpenCodeSessionFor(clientTool, endpointID string) string {
	return openCodeSessionID(openCodeClientTool(clientTool), endpointID)
}

// openCodeSessionID mints or reuses the session for an already-normalised client
// tool, so the cache key and the hashed digest agree on one spelling.
func openCodeSessionID(clientTool, endpointID string) string {
	key := openCodeSessionCachePrefix + clientTool + ":" + endpointID
	return openCodeSessions.getOrMint(key, func(now time.Time, counter uint64) string {
		digest := sha256.Sum256([]byte("opencode\x00" + clientTool + "\x00" + endpointID))
		return "ses_" + openCodeDescendingHex(now.UnixMilli(), counter) + base62(digest[6:20])
	})
}

// openCodeClientTool normalises an unnamed client to the reference's placeholder.
func openCodeClientTool(clientTool string) string {
	if strings.TrimSpace(clientTool) == "" {
		return openCodeGenericClient
	}
	return clientTool
}

// base62 maps each byte into the alphabet by remainder, which is the reference's
// own reduction and needs no rejection sampling: the goal is a canonical-looking
// identity, not a uniform draw.
func base62(bytes []byte) string {
	out := make([]byte, len(bytes))
	for index, value := range bytes {
		out[index] = base62Alphabet[int(value)%len(base62Alphabet)]
	}
	return string(out)
}
