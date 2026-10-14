// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_id_time_test.go
// @for       The clock field OpenCode reads back out of an id's first twelve hex characters.
// @uses      math/big, strconv, strings, testing, time.
// @reason    The upstream decodes that field as `~(ms*0x1000 + counter)` for a session and `ms*0x1000 + 1` for a request, on a BigInt. These tests compute the same two expressions with math/big, an independent formulation, not a copy of the production arithmetic, so the uint64 masking that stands in for arbitrary precision is checked against it rather than against itself.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
)

// bigClockHex is the reference's own arithmetic: place the millisecond and its
// sequence in the field, invert when the id descends, keep the low six bytes.
func bigClockHex(ms int64, counter uint64, invert bool) string {
	field := new(big.Int).SetUint64(uint64(ms))
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
		ms      int64
		counter uint64
	}{
		{"a reference epoch with the first sequence", 1_700_000_000_000, 1},
		{"the same millisecond with a later sequence", 1_700_000_000_000, 7},
		{"a counter that carries into the millisecond bits", 1_700_000_000_000, 5_000},
		{"the live clock", time.Now().UnixMilli(), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := openCodeDescendingHex(tc.ms, tc.counter)
			if len(got) != 12 {
				t.Fatalf("openCodeDescendingHex(%d, %d) = %q, want twelve characters", tc.ms, tc.counter, got)
			}
			if strings.ToLower(got) != got {
				t.Fatalf("openCodeDescendingHex(%d, %d) = %q, want lowercase hex", tc.ms, tc.counter, got)
			}
			if want := bigClockHex(tc.ms, tc.counter, true); want != got {
				t.Fatalf("openCodeDescendingHex(%d, %d) = %s, the reference computes %s", tc.ms, tc.counter, got, want)
			}
		})
	}
}

func TestOpenCodeAscendingHex_MatchesTheBigIntArithmetic(t *testing.T) {
	for _, ms := range []int64{1_700_000_000_000, time.Now().UnixMilli()} {
		if got, want := openCodeAscendingHex(ms), bigClockHex(ms, 1, false); got != want {
			t.Fatalf("openCodeAscendingHex(%d) = %s, the reference computes %s", ms, got, want)
		}
	}
}

func TestOpenCodeClockHex_DescendingIdsSortNewestFirst(t *testing.T) {
	older := openCodeDescendingHex(1_700_000_000_000, 1)
	newer := openCodeDescendingHex(1_700_000_001_000, 1)
	if newer >= older {
		t.Fatalf("a descending session must sort the newer id first: %s is not below %s", newer, older)
	}
}

func TestOpenCodeClockHex_AscendingIdsSortOldestFirst(t *testing.T) {
	older := openCodeAscendingHex(1_700_000_000_000)
	newer := openCodeAscendingHex(1_700_000_001_000)
	if older >= newer {
		t.Fatalf("an ascending message id must sort the older id first: %s is not below %s", older, newer)
	}
}

// TestOpenCodeClockHex_FieldWraps pins the limit a reader is going to trip over:
// the field is 48 bits and `ms*0x1000` passed 2^48 in 2021, so the prefix decodes
// to the low 36 bits of the clock, not the whole millisecond. That is the
// upstream's own scheme, so nothing here should be "fixed" to make it wider.
func TestOpenCodeClockHex_FieldWraps(t *testing.T) {
	ms := time.Now().UnixMilli()
	field, err := strconv.ParseUint(openCodeDescendingHex(ms, 1), 16, 64)
	if err != nil {
		t.Fatalf("decoding the clock field: %v", err)
	}
	clock := (^field & openCodeIDClockMask) / openCodeIDClockScale
	if want := uint64(ms) & (1<<36 - 1); clock != want {
		t.Fatalf("decoded clock = %d, want the low 36 bits of the millisecond (%d)", clock, want)
	}
}
