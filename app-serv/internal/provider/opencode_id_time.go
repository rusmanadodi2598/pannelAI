// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_id_time.go
// @for       The time field OpenCode packs into the first twelve hex characters of its ids.
// @uses      fmt.
// @reason    OpenCode v2 decodes those twelve characters as an identifier clock instead of treating them as opaque bits, so a prefix that is merely hex-shaped is refused with 403 FreeTierError. The field is 48 bits: its low twelve carry a sequence counter, and the upstream rejects a counter past the small values the CLI sends. A session prefix therefore pins the counter to one and derives the 36-bit time from the identity digest (the reference's translateSessionId fix, PR #4842); thirty-six bits is what makes `time*0x1000 + 1` fill the field exactly. That is a property of the upstream's own scheme, not of this port.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import "fmt"

// openCodeIDClockBits is the width of the clock field OpenCode reads back out of
// an id: six bytes, most significant first.
const openCodeIDClockBits = 48

// openCodeIDClockMask keeps only those six bytes.
const openCodeIDClockMask = uint64(1)<<openCodeIDClockBits - 1

// openCodeIDClockScale is the shift between a clock unit and the field: the low
// twelve bits of the field carry a sequence counter.
const openCodeIDClockScale = uint64(0x1000)

// openCodeIDTimeBits is the width of the time inside the clock: the field minus
// the counter's twelve bits.
const openCodeIDTimeBits = openCodeIDClockBits - 12

// openCodeIDTimeMask keeps only the time bits.
const openCodeIDTimeMask = uint64(1)<<openCodeIDTimeBits - 1

// openCodeDigestTime reads the time a session id carries out of the identity
// digest: the low 36 bits of its first five bytes, big-endian, which is the
// reference's `BigInt(digest[0:5]) & SESSION_TIME_MASK`.
func openCodeDigestTime(digest [32]byte) uint64 {
	value := uint64(digest[0])<<32 | uint64(digest[1])<<24 | uint64(digest[2])<<16 |
		uint64(digest[3])<<8 | uint64(digest[4])
	return value & openCodeIDTimeMask
}

// openCodeDescendingHex renders the clock field of a session id, which OpenCode
// packs descending (`Identifier.descending("session")`) as the complement of
// `time*0x1000 + counter` so the newest session sorts first.
//
// The reference computes that complement on a BigInt and reads only its six low
// bytes, so this masks instead of negating: the low 48 bits of a two's-complement
// complement are `2^48-1 - (value mod 2^48)`, which is what a masked uint64
// yields. Negating in int64 would be the wrong width, and would overflow past
// 2^63.
func openCodeDescendingHex(time uint64, counter uint64) string {
	return openCodeClockHex(^openCodeClockValue(time, counter))
}

// openCodeClockValue places a time and its sequence into OpenCode's clock
// field. A counter above 4095 carries into the time bits, which is what the
// reference does too: the field is a single integer to OpenCode, not two.
func openCodeClockValue(time uint64, counter uint64) uint64 {
	return time*openCodeIDClockScale + counter
}

// openCodeClockHex renders the low six bytes of a clock value, big-endian and
// zero padded, which is the twelve lowercase hex characters the upstream parses.
func openCodeClockHex(value uint64) string {
	return fmt.Sprintf("%012x", value&openCodeIDClockMask)
}
