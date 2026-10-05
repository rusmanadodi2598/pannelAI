// Decoder tests for the Grok gRPC-web GetGrokCreditsConfig frame, driven by hand-built byte
// fixtures: valid, truncated, wrong magic, a length that overruns, and an empty buffer.
//
// @file      internal/service/quotafetch/grok_frame_test.go
// @for       Locks the frame decoder's byte layout, bounds checks and soft-failure reasons.
// @uses      internal/service/quotafetch, encoding/binary, math, testing, time
// @reason    The decoder parses untrusted provider bytes; a regression that misreads the magic,
//
//	endianness or a length field would silently corrupt the weekly pool or panic, so the
//	byte layout itself is pinned with fixtures rather than only through the family.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"
)

// grokFrameTag encodes one protobuf field tag: (number<<3)|wire, as a base-128 varint.
func grokFrameTag(number, wire int) []byte {
	return grokFrameVarint(uint64(number)<<3 | uint64(wire))
}

func grokFrameVarint(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func grokFrameLengthDelimited(number int, body []byte) []byte {
	out := grokFrameTag(number, 2)
	out = append(out, grokFrameVarint(uint64(len(body)))...)
	return append(out, body...)
}

func grokFrameFixed32(number int, bits float32) []byte {
	out := grokFrameTag(number, 5)
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(bits))
	return append(out, buf[:]...)
}

// grokFrameBuildCredits assembles the nested credits message: usage ratio (fixed32) plus a
// Timestamp (seconds/nanos varints), wrapped at top-level field 1 and gRPC-web framed.
func grokFrameBuildCredits(ratio float32, seconds, nanos uint64, withReset bool) []byte {
	inner := grokFrameFixed32(grokCreditsFieldUsageRatio, ratio)
	if withReset {
		var ts []byte
		ts = append(ts, grokFrameTag(grokTimestampFieldSeconds, 0)...)
		ts = append(ts, grokFrameVarint(seconds)...)
		ts = append(ts, grokFrameTag(grokTimestampFieldNanos, 0)...)
		ts = append(ts, grokFrameVarint(nanos)...)
		inner = append(inner, grokFrameLengthDelimited(grokCreditsFieldReset, ts)...)
	}
	return grokFrameWrap(grokFrameLengthDelimited(grokFrameFieldCreditsInfo, inner))
}

// grokFrameWrap prefixes a gRPC-web data frame: flag byte 0x00 and a big-endian payload length.
func grokFrameWrap(payload []byte) []byte {
	frame := []byte{0x00}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	return append(append(frame, lenBuf[:]...), payload...)
}

func TestGrokFrame_ValidFrameYieldsRatioAndReset(t *testing.T) {
	seconds := uint64(time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC).Unix())
	data := grokFrameBuildCredits(0.25, seconds, 500_000_000, true)

	decoded, err := grokFrameDecode(data)
	if err != nil {
		t.Fatalf("valid frame failed to decode: %v", err)
	}
	if decoded.PercentUsed != 25 {
		t.Errorf("percent = %v, want 25", decoded.PercentUsed)
	}
	want := time.UnixMilli(int64(seconds)*1000 + 500).UTC()
	if !decoded.HasReset || !decoded.ResetAt.Equal(want) {
		t.Errorf("reset = %v (%v), want %v", decoded.ResetAt, decoded.HasReset, want)
	}
}

func TestGrokFrame_AcceptsBareProtobufPayload(t *testing.T) {
	// The reference fail-opens to a bare message when the buffer is not gRPC-web framed.
	inner := grokFrameFixed32(grokCreditsFieldUsageRatio, 0.5)
	bare := grokFrameLengthDelimited(grokFrameFieldCreditsInfo, inner)

	decoded, err := grokFrameDecode(bare)
	if err != nil || decoded.PercentUsed != 50 || decoded.HasReset {
		t.Fatalf("bare payload = %+v err=%v, want percent 50 and no reset", decoded, err)
	}
}

func TestGrokFrame_SoftFailsOnMalformedInput(t *testing.T) {
	valid := grokFrameBuildCredits(0.1, 1_700_000_000, 0, true)

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{name: "empty buffer", data: []byte{}, want: errGrokFrameEmpty},
		{name: "wrong magic", data: []byte{0x02, 0x00, 0x00, 0x00, 0x01, 0x01}, want: errGrokFrameShape},
		{name: "length overruns buffer", data: []byte{0x00, 0x00, 0x00, 0x7f, 0xff, 0x0a, 0x01}, want: errGrokFrameOverrun},
		{name: "truncated header", data: []byte{0x00, 0x00, 0x00}, want: errGrokFrameShort},
		{name: "truncated payload", data: valid[:len(valid)-2], want: errGrokFrameOverrun},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := grokFrameDecode(testCase.data)
			if err == nil {
				t.Fatalf("expected a soft failure, got none")
			}
			if !errors.Is(err, testCase.want) {
				t.Fatalf("err = %v, want %v", err, testCase.want)
			}
		})
	}
}

// A declared length larger than the safety cap is rejected outright, not trusted.
func TestGrokFrame_OversizedLengthIsCapped(t *testing.T) {
	frame := []byte{0x00}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(grokFrameMaxPayload+1))
	frame = append(frame, lenBuf[:]...)
	frame = append(frame, make([]byte, grokFrameMaxPayload)...)

	if _, err := grokFrameDecode(frame); !errors.Is(err, errGrokFrameOversize) && !errors.Is(err, errGrokFrameOverrun) {
		t.Fatalf("oversized frame err = %v, want oversize/overrun", err)
	}
}

// TestGrokFrame_AbsurdEpochReportsNoReset pins the overflow guard. A varint epoch past
// what a calendar can hold would wrap when turned into milliseconds, and the card would
// print a refill date from the wrong millennium instead of admitting the frame was
// nonsense. Reporting no reset is the honest answer: the ratio still arrived.
func TestGrokFrame_AbsurdEpochReportsNoReset(t *testing.T) {
	var inner []byte
	inner = append(inner, 0x08) // Timestamp field 1 (seconds), varint wire type
	var scratch [10]byte
	inner = append(inner, scratch[:binary.PutUvarint(scratch[:], 1<<63)]...)

	credits := []byte{byte(5<<3) | 2, byte(len(inner))} // field 5, length-delimited
	credits = append(credits, inner...)

	if _, hasReset := grokFrameReset(credits); hasReset {
		t.Fatal("an epoch of 2^63 seconds was accepted as a reset instant")
	}

	// The guard must not reject a real instant: a window closing in 2100 is absurd for
	// a quota, but it is a date a provider could actually state.
	var plausible []byte
	plausible = append(plausible, 0x08)
	plausible = append(plausible, scratch[:binary.PutUvarint(scratch[:], 4_102_444_800)]...)
	nested := []byte{byte(5<<3) | 2, byte(len(plausible))}
	nested = append(nested, plausible...)
	if _, hasReset := grokFrameReset(nested); !hasReset {
		t.Fatal("a plausible epoch was refused; only the implausible one should be dropped")
	}
}
