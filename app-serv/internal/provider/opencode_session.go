// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session.go
// @for       The canonical OpenCode session identity a free-tier request carries.
// @uses      crypto/sha256, regexp, strings.
// @reason    The free tier refuses any value that is not the CLI's own canonical shape; its quota is counted per egress address rather than per session (measured 2026-10-15, SPEC-API-001 §7.11), so this file is about the shape alone. The twelve hex characters are decoded as `~(time*0x1000 + counter)`, and the upstream rejects a counter past the small values the CLI sends with 403 FreeTierError, which a digest-filled 48-bit prefix produced about 97% of the time. The port pins the counter to the one the CLI sends and derives the 36-bit time from the identity digest (the reference's translateSessionId fix, PR #4842), so one identity still presents one stable session.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-09-21
package provider

import (
	"crypto/sha256"
	"regexp"
	"strings"
)

// OpenCodeSessionRE is the session shape the upstream accepts:
// `ses_` followed by twelve lowercase hex digits and fourteen base62
// characters (the reference's own OPENCODE_SESSION_RE). Any other
// value is refused with 403, so the connector mints only this shape.
var OpenCodeSessionRE = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)

// base62Alphabet is the reference's alphabet, in its order.
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// openCodeGenericClient is what the reference substitutes when no client tool
// names itself, so a session minted for an anonymous client is a different
// identity from one minted for a named one.
const openCodeGenericClient = "generic"

// openCodeIDSuffixLen is the base62 identity tail of an OpenCode id: fourteen
// characters taken from the identity digest.
const openCodeIDSuffixLen = 14

// OpenCodeSession derives the canonical session id one endpoint presents. It is
// stable per identity: the same endpoint presents the same session across every
// request, so its quota is not spent on fresh sessions, and two endpoints hash to
// different ids, so one caller's quota is never spent by another's traffic.
//
// The layout mirrors the reference's `translateSessionId`. The twelve hex
// characters carry the low 36 bits of the identity digest as the decoded time
// field with the counter pinned to one, and the fourteen base62 characters
// carry the rest of the digest, which is what makes one endpoint's session its
// own and never another's.
func OpenCodeSession(endpointID string) string {
	return openCodeSessionID(openCodeClientTool(""), endpointID)
}

// OpenCodeSessionFor presents the session belonging to one client tool on one
// endpoint, which is the pair the reference keys its own store by.
func OpenCodeSessionFor(clientTool, endpointID string) string {
	return openCodeSessionID(openCodeClientTool(clientTool), endpointID)
}

// openCodeSessionID derives the session for an already-normalised client tool,
// so the digest and the cache key agree on one spelling. It is a pure function
// of the pair: the upstream accepts a digest-derived prefix only when its
// counter is the small value the CLI sends, and pinning it there costs the
// identity nothing because the time field is deterministic too.
func openCodeSessionID(clientTool, endpointID string) string {
	digest := sha256.Sum256([]byte("opencode\x00" + clientTool + "\x00" + endpointID))
	return "ses_" + openCodeDescendingHex(openCodeDigestTime(digest), 1) + base62(digest[6:6+openCodeIDSuffixLen])
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
