// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session.go
// @for       The canonical OpenCode session identity a free-tier request carries.
// @uses      crypto/sha256, encoding/hex, strings.
// @reason    The free tier accounts quota per session and refuses any value that
//
//	is not the CLI's own canonical shape, so a gateway that mints a
//	random id per request burns the quota it is trying to spend (the
//	reference fixed exactly that: one stable session per identity, commit
//	0c6ab4f9). Deriving it from the endpoint id makes it stable without
//	the connector holding per-request state, which it must not: a
//	connector is built once and shared by concurrent requests.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-21
package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// OpenCodeSessionRE is the session shape the upstream accepts:
// `ses_` followed by twelve lowercase hex digits and fourteen base62
// characters (the reference's OPENCODE_SESSION_RE, commit 6091ff59). Any other
// value is refused with 403, so the connector mints only this shape.
var OpenCodeSessionRE = regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)

// base62Alphabet is the reference's alphabet, in its order.
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// OpenCodeSession derives the canonical session id one endpoint presents.
//
// It is a pure function of the endpoint id, which is what makes it both stable
// (the same account presents one identity across every request, so its quota is
// not spent on fresh sessions) and stateless (the connector holds no map and
// needs no lock). Two endpoints hash to different ids, so one caller's quota is
// never spent by another's traffic.
//
// The layout mirrors the reference's `translateSessionId`: the first six digest
// bytes render as lowercase hex and the next fourteen map into base62, giving
// the twelve-hex-plus-fourteen-base62 shape the regex above accepts.
func OpenCodeSession(endpointID string) string {
	digest := sha256.Sum256([]byte("opencode\x00" + endpointID))
	return "ses_" + hex.EncodeToString(digest[:6]) + base62(digest[6:20])
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
