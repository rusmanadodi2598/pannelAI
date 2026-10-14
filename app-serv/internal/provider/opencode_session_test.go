// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session_test.go
// @for       The session identity one endpoint presents to the OpenCode free tier.
// @uses      crypto/sha256, math/big, regexp, strconv, testing.
// @reason    Two properties keep the free tier working: the id is stable per identity, so one account is not charged for a fresh session on every request, and its first twelve characters decode as `~(time*0x1000 + counter)` with a counter the upstream accepts, which is what OpenCode v2 validates and refuses with 403 when the counter is not the small value the CLI sends. The shape and the stability were pinned before; the pinned counter is what the outage was missing.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"crypto/sha256"
	"math/big"
	"regexp"
	"strconv"
	"testing"
)

// openCodeClockFieldRE cuts an id down to the twelve characters the upstream
// decodes, leaving the base62 identity tail behind.
var openCodeClockFieldRE = regexp.MustCompile(`^ses_([0-9a-f]{12})[0-9A-Za-z]{14}$`)

// openCodeClockOf decodes an id's clock field back into the value the upstream
// reads: `~(field)` carries `time*0x1000 + counter`.
func openCodeClockOf(t *testing.T, id string) uint64 {
	t.Helper()
	parts := openCodeClockFieldRE.FindStringSubmatch(id)
	if parts == nil {
		t.Fatalf("id %q does not carry a decodable clock field", id)
	}
	field, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		t.Fatalf("decoding the clock field of %q: %v", id, err)
	}
	return ^field & openCodeIDClockMask
}

func TestOpenCodeSession_GivesTheCanonicalShape(t *testing.T) {
	cases := []struct {
		name       string
		clientTool string
		endpointID string
	}{
		{"the free lane's virtual endpoint", "", "virtual:opencode"},
		{"a named client tool", "desktop", "virtual:opencode"},
		{"a systemone identity", "", "virtual:opencode:systemone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := OpenCodeSessionFor(tc.clientTool, tc.endpointID)
			if !OpenCodeSessionRE.MatchString(id) {
				t.Fatalf("OpenCodeSessionFor(%q, %q) = %q, does not match %s",
					tc.clientTool, tc.endpointID, id, OpenCodeSessionRE)
			}
		})
	}
}

// TestOpenCodeSession_PinsTheCounterAndTheDigestTime pins the fix: the upstream
// decodes the prefix as `~(time*0x1000 + counter)` and answers a non-small
// counter with 403 FreeTierError, which a digest-filled 48-bit prefix produced
// about 97% of the time. The port pins the counter to the one the CLI sends and
// derives the time from the identity digest, so the id is a pure function of
// the identity pair.
func TestOpenCodeSession_PinsTheCounterAndTheDigestTime(t *testing.T) {
	id := OpenCodeSessionFor("cli", "virtual:opencode")
	clock := openCodeClockOf(t, id)

	if counter := clock % openCodeIDClockScale; counter != 1 {
		t.Fatalf("session %q decodes to counter %d, want the pinned 1", id, counter)
	}
	digest := sha256.Sum256([]byte("opencode\x00cli\x00virtual:opencode"))
	want := new(big.Int).SetUint64(openCodeDigestTime(digest))
	if got := new(big.Int).SetUint64(clock / openCodeIDClockScale); got.Cmp(want) != 0 {
		t.Fatalf("session %q decodes to time %s, want the digest's %s", id, got, want)
	}
}

func TestOpenCodeSession_StaysOneIdentityAndSeparatesOthers(t *testing.T) {
	first := OpenCodeSessionFor("desktop", "virtual:opencode")
	if again := OpenCodeSessionFor("desktop", "virtual:opencode"); again != first {
		t.Fatalf("one identity minted twice: %s then %s", first, again)
	}
	if other := OpenCodeSessionFor("cli", "virtual:opencode"); other == first {
		t.Fatal("two client tools share one session, so one spends the other's quota")
	}
	if other := OpenCodeSessionFor("desktop", "virtual:opencode-zen"); other == first {
		t.Fatal("two endpoints share one session, so one spends the other's quota")
	}
}

func TestOpenCodeSession_UnnamedClientIsTheGenericIdentity(t *testing.T) {
	// The reference substitutes "generic" for a client tool that does not name
	// itself, and the legacy single-argument entry point keeps that spelling so a
	// caller outside the connector still lands on the same identity.
	if blank, named := OpenCodeSession("virtual:opencode"), OpenCodeSessionFor("generic", "virtual:opencode"); blank != named {
		t.Fatalf("the unnamed client presented %s while \"generic\" presented %s", blank, named)
	}
}

func TestOpenCodeSession_TailCarriesTheIdentityNotTheClock(t *testing.T) {
	desktop := OpenCodeSessionFor("desktop", "virtual:opencode")
	if tail := desktop[len("ses_")+12:]; len(tail) != openCodeIDSuffixLen {
		t.Fatalf("id %q carries a %d-character identity tail, want %d", desktop, len(tail), openCodeIDSuffixLen)
	}
	if other := OpenCodeSessionFor("desktop", "virtual:opencode-go"); other[:12] == desktop[:12] && other[12:] == desktop[12:] {
		t.Fatalf("two endpoints produced one identical id: %s", other)
	}
}
