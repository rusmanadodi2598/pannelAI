// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_id_time_test.go
// @for       The time field OpenCode reads back out of an id's first twelve hex characters.
// @uses      crypto/sha256, fmt, math/big, strings, testing.
// @reason    The upstream decodes that field as `~(time*0x1000 + counter)` on a BigInt and refuses a counter past the small values the CLI sends with 403 FreeTierError, which is why the session prefix pins the counter to one and derives the 36-bit time from the identity digest. These tests compute the same expression with math/big, an independent formulation, not a copy of the production arithmetic, so the uint64 masking that stands in for arbitrary precision is checked against it rather than against itself.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
)

// bigClockHex is the reference's own arithmetic: place the time and its
// sequence in the field, invert when the id descends, keep the low six bytes.
func bigClockHex(time uint64, counter uint64, invert bool) string {
	field := new(big.Int).SetUint64(time)
	field.Mul(field, new(big.Int).SetUint64(openCodeIDClockScale))
	field.Add(field, new(big.Int).SetUint64(counter))
	if invert {
		field.Not(field)
	}
	field.And(field, new(big.Int).SetUint64(openCodeIDClockMask))
	// The reference pads every byte to two hex characters, which is what keeps a
	// clock whose top byte is zero twelve characters wide.
	return fmt.Sprintf("%012x", field)
}

func TestOpenCodeDescendingHex_MatchesTheBigIntArithmetic(t *testing.T) {
	cases := []struct {
		name    string
		time    uint64
		counter uint64
	}{
		{"the smallest field", 0, 1},
		{"a digest-shaped time with the pinned counter", 0x9f3c2a1b4d, 1},
		{"the largest 36-bit time", openCodeIDTimeMask, 1},
		{"a counter that carries into the time bits", 0x9f3c2a1b4d, 5_000},
		{"a clock value the field can only carry masked", 1_700_000_000_000, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := openCodeDescendingHex(tc.time, tc.counter)
			if len(got) != 12 {
				t.Fatalf("openCodeDescendingHex(%d, %d) = %q, want twelve characters", tc.time, tc.counter, got)
			}
			if strings.ToLower(got) != got {
				t.Fatalf("openCodeDescendingHex(%d, %d) = %q, want lowercase hex", tc.time, tc.counter, got)
			}
			if want := bigClockHex(tc.time, tc.counter, true); want != got {
				t.Fatalf("openCodeDescendingHex(%d, %d) = %s, the reference computes %s", tc.time, tc.counter, got, want)
			}
		})
	}
}

// TestOpenCodeDigestTime_ReadsTheLow36BitsOfTheFirstFiveBytes pins the
// reference's `BigInt(digest[0:5]) & SESSION_TIME_MASK` against a big.Int
// computation of the same expression.
func TestOpenCodeDigestTime_ReadsTheLow36BitsOfTheFirstFiveBytes(t *testing.T) {
	digest := sha256.Sum256([]byte("opencode\x00cli\x00virtual:opencode"))
	five := new(big.Int).SetBytes(digest[:5])
	want := new(big.Int).And(five, new(big.Int).SetUint64(openCodeIDTimeMask))
	if got := openCodeDigestTime(digest); new(big.Int).SetUint64(got).Cmp(want) != 0 {
		t.Fatalf("openCodeDigestTime = %d, the reference computes %s", got, want)
	}
	if openCodeIDTimeBits != 36 {
		t.Fatalf("the time field is %d bits, want the reference's 36", openCodeIDTimeBits)
	}
}

func TestOpenCodeClockHex_DescendingIdsSortNewestFirst(t *testing.T) {
	older := openCodeDescendingHex(0x9f3c2a1b4d, 1)
	newer := openCodeDescendingHex(0x9f3c2a1b4e, 1)
	if newer >= older {
		t.Fatalf("a descending session must sort the newer id first: %s is not below %s", newer, older)
	}
}

// TestOpenCodeClockHex_TheDigestPrefixFillsTheFieldExactly pins the shape the
// fix produces: a 36-bit digest time shifted by the counter's twelve bits fills
// the 48-bit field with no wrap, and the counter decodes to the one the CLI
// sends. That is what the upstream checks before it accepts a free-tier session.
func TestOpenCodeClockHex_TheDigestPrefixFillsTheFieldExactly(t *testing.T) {
	digest := sha256.Sum256([]byte("opencode\x00cli\x00virtual:opencode"))
	time := openCodeDigestTime(digest)
	field, err := strconv.ParseUint(openCodeDescendingHex(time, 1), 16, 64)
	if err != nil {
		t.Fatalf("decoding the clock field: %v", err)
	}
	clock := ^field & openCodeIDClockMask
	if counter := clock % openCodeIDClockScale; counter != 1 {
		t.Fatalf("decoded counter = %d, want the pinned 1", counter)
	}
	if decoded := clock / openCodeIDClockScale; decoded != time {
		t.Fatalf("decoded time = %d, want the digest's %d", decoded, time)
	}
	if field == 0 {
		t.Fatal("the field is all zeroes, want the complement of the digest time")
	}
}
