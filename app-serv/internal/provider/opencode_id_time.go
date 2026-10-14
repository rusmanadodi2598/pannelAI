// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_id_time.go
// @for       The clock field OpenCode packs into the first twelve hex characters of its ids.
// @uses      fmt.
// @reason    OpenCode 1.19.0 decodes those twelve characters as an identifier timestamp instead of treating them as opaque bits, so a prefix that is merely hex-shaped is refused with 403 FreeTierError. The field is 48 bits wide and `Date.now()*0x1000` already exceeds it, so the value wraps: a decoded timestamp is only correct modulo 2^36. That is a property of the upstream's own scheme, not of this port, and callers must not read the prefix as a full timestamp.
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

// openCodeIDClockScale is the shift between a millisecond and OpenCode's clock
// unit: the low twelve bits of the field carry a sequence counter.
const openCodeIDClockScale = uint64(0x1000)

// openCodeDescendingHex renders the clock field of a session id. OpenCode builds
// session identifiers descending (`Identifier.descending("session")`), which packs
// the bitwise complement of `ms*0x1000 + counter`, so the newest session sorts
// first.
//
// The reference computes that complement on an arbitrary-precision BigInt and
// then reads only the six low bytes of it. Those two facts are why this is a
// mask rather than a negative number: the low 48 bits of a two's-complement
// complement are exactly `2^48-1 - (value mod 2^48)`, which is what masking a
// uint64 complement yields. Writing `-(value+1)` in int64 would be wrong twice
// over: the wrong width, and an overflow once the millisecond counter passes
// 2^63.
func openCodeDescendingHex(ms int64, counter uint64) string {
	return openCodeClockHex(^openCodeClockValue(ms, counter))
}

// openCodeAscendingHex renders the clock field of a request id, which OpenCode
// builds ascending (`Identifier.ascending("message")`) with a fixed sequence of
// one: the first id minted for a message.
func openCodeAscendingHex(ms int64) string {
	return openCodeClockHex(openCodeClockValue(ms, 1))
}

// openCodeClockValue places a millisecond and its sequence into OpenCode's clock
// field. A counter above 4095 carries into the millisecond bits, which is what
// the reference does too, the field is a single integer to OpenCode, not two.
func openCodeClockValue(ms int64, counter uint64) uint64 {
	return uint64(ms)*openCodeIDClockScale + counter
}

// openCodeClockHex renders the low six bytes of a clock value, big-endian and
// zero padded, which is the twelve lowercase hex characters the upstream parses.
func openCodeClockHex(value uint64) string {
	return fmt.Sprintf("%012x", value&openCodeIDClockMask)
}
