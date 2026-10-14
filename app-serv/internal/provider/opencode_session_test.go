// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_session_test.go
// @for       The session identity one endpoint presents to the OpenCode free tier.
// @uses      regexp, strconv, strings, testing, time.
// @reason    Two properties keep the free tier working: the id is stable per identity, so one account is not charged for a fresh session on every request, and its first twelve characters decode as a clock reading, which is what OpenCode 1.19.0 validates and refuses with 403 when they are only hex-shaped. The shape was pinned before; the decodable clock is what the outage was missing.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"regexp"
	"strconv"
	"testing"
	"time"
)

// openCodeClockDriftMs is how far a remembered session may legitimately sit from
// "now" in a test run that just started: a minute is generous, six years is what
// a random prefix would give.
const openCodeClockDriftMs = 60_000

// openCodeClockFieldRE cuts an id down to the twelve characters the upstream
// decodes, leaving the base62 identity tail behind.
var openCodeClockFieldRE = regexp.MustCompile(`^(?:ses|msg)_([0-9a-f]{12})[0-9A-Za-z]{14}$`)

// openCodeClockOf decodes an id's clock field back into the millisecond it
// carries, which is only the low 36 bits because the field is 48 wide.
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
	return (^field & openCodeIDClockMask) / openCodeIDClockScale
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

func TestOpenCodeSession_ClockFieldDecodesToTheMintTime(t *testing.T) {
	now := time.Now().UnixMilli()
	id := OpenCodeSessionFor("desktop", "virtual:opencode")
	decoded := int64(openCodeClockOf(t, id))
	wanted := now & (1<<36 - 1)

	// The session may have been minted by an earlier request in this process, 	// that is the point of remembering it, so the test asks that the decoded
	// clock sit inside the last minute rather than at this exact millisecond. A
	// SHA-256 byte string, which is what this field carried before the fix,
	// lands anywhere across the 2^36 milliseconds the field can hold: roughly
	// six years away from now, and read by the upstream as a timestamp from no
	// plausible era.
	if drift := wanted - decoded; drift > openCodeClockDriftMs || drift < -openCodeClockDriftMs {
		t.Fatalf("session %q decodes to a clock %d ms from now, want it minted within the last minute",
			id, drift)
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
