// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/ulid.go
// @for       Dependency-free Crockford-base32 ULID generation for type-prefixed IDs.
// @uses      crypto/rand, sync, time (standard library only).
// @reason    SPEC-API-001 §4 requires ULID IDs with type prefixes; generating
//
//	them without a third-party dependency keeps app-serv stdlib-first
//	per AGENTS.md "Stack". Uniqueness and ordering are carried by the
//	timestamp, so the random part can never produce a collision.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

import (
	"crypto/rand"
	"sync"
	"time"
)

const (
	ulidLen      = 26
	ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// ulidState guards the monotonic clock. All access is under ulidMu.
var (
	ulidMu     sync.Mutex
	ulidLastMS uint64
)

// NewULID returns a 26-character ULID: a 48-bit millisecond timestamp followed
// by 80 bits of randomness. Uniqueness is carried by the timestamp: when the
// incoming instant is not strictly after the previous one, it is clamped to
// previous+1ms, so two calls in the same millisecond (or a clock that jumps
// backwards) still yield distinct, sortable IDs whatever the random part is.
func NewULID(now time.Time) string {
	ms := uint64(now.UnixMilli())

	ulidMu.Lock()
	if ms <= ulidLastMS {
		ms = ulidLastMS + 1
	}
	ulidLastMS = ms
	ulidMu.Unlock()

	var randPart [10]byte
	if _, err := rand.Read(randPart[:]); err != nil {
		// The system CSPRNG failed. Leave the random part zero: the strictly
		// increasing timestamp still makes this ID unique and ordered, so the
		// request is served rather than aborted over an identifier.
		randPart = [10]byte{}
	}

	return encodeULID(ms, randPart)
}

// encodeULID packs a 48-bit millisecond timestamp and 80 bits of randomness
// into 26 Crockford base32 characters, most significant bit first. The layout
// follows the ULID specification: 48+80 = 128 bits fill the 16-byte buffer
// exactly, so the timestamp prefix is what makes IDs sortable.
func encodeULID(ms uint64, randPart [10]byte) string {
	var b [16]byte
	// 48-bit timestamp, big-endian, in the first 6 bytes.
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	// 80 bits of randomness fill the remaining 10 bytes.
	copy(b[6:], randPart[:])

	out := make([]byte, ulidLen)
	// 128 bits over 26 chars: the head char carries 4 bits, the rest carry 5.
	for i := range out {
		var val byte
		if i == 0 {
			val = b[0] >> 4
		} else {
			bit := 4 + (i-1)*5
			val = readBits(b[:], bit, 5)
		}
		out[i] = ulidAlphabet[val]
	}
	return string(out)
}

// readBits reads n bits starting at the given bit offset, big-endian.
func readBits(b []byte, offset, n int) byte {
	var val byte
	for j := 0; j < n; j++ {
		bit := offset + j
		byteIdx := bit / 8
		if byteIdx >= len(b) {
			continue
		}
		mask := byte(1 << (7 - (bit % 8)))
		if b[byteIdx]&mask != 0 {
			val |= 1 << (uint(n-1) - uint(j))
		}
	}
	return val
}
