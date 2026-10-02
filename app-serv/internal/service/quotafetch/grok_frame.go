// Grok CLI's binary wire path: the gRPC-web frame decoder for GetGrokCreditsConfig.
// A malformed frame fails soft — never a panic.
//
// @file      internal/service/quotafetch/grok_frame.go
// @for       Decodes one GetGrokCreditsConfig gRPC-web answer into a usage ratio and a reset instant.
// @uses      encoding/binary, errors, math, time
// @reason    Grok publishes its weekly pool only as a binary protobuf frame, over bytes this
//
//	gateway does not control, so every length is bounds-checked against what
//	remains before it is sliced, and any cap the frame declares is capped again here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"encoding/binary"
	"errors"
	"math"
	"time"
)

const (
	grokFrameFieldCreditsInfo  = 1 // top-level length-delimited credits pool
	grokCreditsFieldUsageRatio = 1 // fixed32 (or fixed64) ratio, 0..1
	grokCreditsFieldReset      = 5 // length-delimited Timestamp
	grokTimestampFieldSeconds  = 1
	grokTimestampFieldNanos    = 2
	grokWireVarint             = 0
	grokWireFixed64            = 1
	grokWireLengthDelimited    = 2
	grokWireFixed32            = 5
	grokFrameTrailerFlagBit    = 0x80
	grokFrameHeaderWidth       = 5
	grokFrameMaxVarintBytes    = 10
	grokFrameMaxPayload        = 1 << 20 // 1 MiB caps any provider-declared length; unbounded it would trust a truncated buffer
)

var errGrokFrameEmpty, errGrokFrameShort, errGrokFrameOverrun = errors.New("frame buffer is empty"), errors.New("frame header is truncated"), errors.New("frame payload overruns the buffer")
var errGrokFrameOversize, errGrokFrameShape = errors.New("frame payload length exceeds the safety cap"), errors.New("frame carries no decodable credits message")

type grokFrameResult struct {
	PercentUsed float64
	ResetAt     time.Time
	HasReset    bool
}

// grokFrameField is one scanned field, a concrete struct (never boxed into interface{}).
type grokFrameField struct {
	number, wireType int
	value            uint64
	bytes            []byte
}

// grokFrameDecode turns the raw body into a usage ratio and reset, accepting a framed gRPC-web body
// or a bare protobuf payload, and names an error for an empty, mis-magicked, truncated or oversized one.
func grokFrameDecode(data []byte) (grokFrameResult, error) {
	if len(data) == 0 {
		return grokFrameResult{}, errGrokFrameEmpty
	}
	payload, err := grokFramePayload(data)
	if err != nil {
		return grokFrameResult{}, err
	}
	credits, ok, err := grokFrameFind(payload, grokFrameFieldCreditsInfo)
	if err != nil {
		return grokFrameResult{}, err
	}
	if !ok || credits.wireType != grokWireLengthDelimited {
		return grokFrameResult{}, errGrokFrameShape
	}
	percent, err := grokFrameRatio(credits.bytes)
	if err != nil {
		return grokFrameResult{}, err
	}
	resetAt, hasReset := grokFrameReset(credits.bytes)
	return grokFrameResult{PercentUsed: percent, ResetAt: resetAt, HasReset: hasReset}, nil
}

// grokFramePayload returns the protobuf bytes inside the response: a valid first flag byte unwraps
// frame by frame, skipping trailers, to the first data frame, bounds-checking the length first.
func grokFramePayload(data []byte) ([]byte, error) {
	switch data[0] {
	case 0x00, 0x01, 0x80, 0x81:
	default:
		return data, nil
	}
	for offset := 0; offset < len(data); {
		if len(data)-offset < grokFrameHeaderWidth {
			return nil, errGrokFrameShort
		}
		length := int(binary.BigEndian.Uint32(data[offset+1 : offset+grokFrameHeaderWidth]))
		switch {
		case length > len(data)-(offset+grokFrameHeaderWidth):
			return nil, errGrokFrameOverrun
		case length > grokFrameMaxPayload:
			return nil, errGrokFrameOversize
		}
		start := offset + grokFrameHeaderWidth
		if data[offset]&grokFrameTrailerFlagBit == 0 {
			return data[start : start+length], nil
		}
		offset = start + length
	}
	return nil, errGrokFrameOverrun
}

// grokFrameFind walks one protobuf message and returns the LAST field with want, matching the
// reference's last-wins Map. Every length is checked against the remaining buffer before slicing,
// and a bad wire type or field number 0 fails the scan, so a malformed field fails the decode.
func grokFrameFind(buf []byte, want int) (grokFrameField, bool, error) {
	var found grokFrameField
	present := false
	for offset := 0; offset < len(buf); {
		tag, next, ok := grokFrameReadVarint(buf, offset)
		if !ok {
			return found, present, errGrokFrameShort
		}
		number, wire := int(tag>>3), int(tag&0x7)
		if number == 0 {
			return found, present, errGrokFrameShape
		}
		field := grokFrameField{number: number, wireType: wire}
		switch wire {
		case grokWireVarint:
			value, after, ok := grokFrameReadVarint(buf, next)
			if !ok {
				return found, present, errGrokFrameShort
			}
			field.value, offset = value, after
		case grokWireFixed32, grokWireFixed64:
			width := 4
			if wire == grokWireFixed64 {
				width = 8
			}
			if next+width > len(buf) {
				return found, present, errGrokFrameShort
			}
			field.bytes, offset = buf[next:next+width], next+width
		case grokWireLengthDelimited:
			length, after, ok := grokFrameReadVarint(buf, next)
			if !ok {
				return found, present, errGrokFrameShort
			}
			if length > grokFrameMaxPayload {
				return found, present, errGrokFrameOversize
			}
			if uint64(len(buf)-after) < length {
				return found, present, errGrokFrameOverrun
			}
			field.bytes, offset = buf[after:after+int(length)], after+int(length)
		default:
			return found, present, errGrokFrameShape
		}
		if number == want {
			found, present = field, true
		}
	}
	return found, present, nil
}

func grokFrameReadVarint(buf []byte, offset int) (uint64, int, bool) {
	var result uint64
	var shift uint
	for i := 0; i < grokFrameMaxVarintBytes; i++ {
		if offset >= len(buf) {
			return 0, 0, false
		}
		byteValue := buf[offset]
		offset++
		result |= uint64(byteValue&0x7f) << shift
		if byteValue < 0x80 {
			return result, offset, true
		}
		shift += 7
	}
	return 0, 0, false
}

// grokFrameRatio reads the usage ratio: absent is 0 (proto3 omission), fixed32/64 a little-endian
// float, and a negative or non-finite ratio is undecodable. Clamped to 100%.
func grokFrameRatio(buf []byte) (float64, error) {
	field, present, err := grokFrameFind(buf, grokCreditsFieldUsageRatio)
	if err != nil {
		return 0, err
	}
	ratio := float64(0)
	if present {
		switch field.wireType {
		case grokWireFixed32:
			ratio = float64(math.Float32frombits(binary.LittleEndian.Uint32(field.bytes)))
		case grokWireFixed64:
			ratio = math.Float64frombits(binary.LittleEndian.Uint64(field.bytes))
		default:
			return 0, errGrokFrameShape
		}
	}
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 {
		return 0, errGrokFrameShape
	}
	return min(100, ratio*100), nil
}

// roundGrokPercent prints the ratio the way the card reads it: a whole percentage of a
// hundred-unit window, never a fraction of one.
func roundGrokPercent(ratio float64) float64 { return math.Round(ratio) }

// grokFrameReset reads the nested Timestamp into millisecond precision; a bad shape reports no reset, the ratio still usable.
func grokFrameReset(buf []byte) (time.Time, bool) {
	field, present, err := grokFrameFind(buf, grokCreditsFieldReset)
	if err != nil || !present || field.wireType != grokWireLengthDelimited {
		return time.Time{}, false
	}
	seconds, _, errS := grokFrameFind(field.bytes, grokTimestampFieldSeconds)
	nanos, _, errN := grokFrameFind(field.bytes, grokTimestampFieldNanos)
	if errS != nil || errN != nil {
		return time.Time{}, false
	}
	// A uint64 varint read from the wire is not a sane epoch: converting one above
	// max int64 wraps it, and a card would print a reset date from the wrong
	// millennium rather than admit the frame made no sense. Nothing refills a quota
	// window this side of the year 9999.
	const grokFrameMaxEpochSeconds = int64(253402300800) // 9999-12-31T23:59:59Z
	if seconds.value > uint64(grokFrameMaxEpochSeconds) {
		return time.Time{}, false
	}
	millis := int64(seconds.value)*1000 + int64(math.Round(float64(nanos.value)/1e6))
	return time.UnixMilli(millis).UTC(), true
}
