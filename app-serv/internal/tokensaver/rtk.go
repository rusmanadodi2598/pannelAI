// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/rtk.go
// @for       The RTK pass: finding the tool results in an upstream body and
//
//	handing each one to the filter that claims it.
//
// @uses      encoding/json, fmt.
// @reason    SPEC-API-002 §3 makes tool results the only thing RTK touches: a
//
//	user's own words are never rewritten, and a tool result that reports
//	an error is preserved so the model can see the failure. Both rules
//	are structural here rather than a filter's responsibility.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"encoding/json"
	"fmt"
)

// Stats reports what one RTK pass measured, in the form a log line needs.
type Stats struct {
	// BytesBefore is the size of every compressible blob the pass inspected.
	BytesBefore int
	// BytesAfter is what those blobs weigh after the pass.
	BytesAfter int
	// Hits names each blob that was actually rewritten.
	Hits []Hit
}

// Hit is one rewritten blob.
type Hit struct {
	// Shape is the wire shape the blob was found in, e.g. "claude-array".
	Shape string
	// Filter is the filter that claimed it.
	Filter string
	// Saved is the byte count the rewrite removed.
	Saved int
}

// Saved reports the total byte count the pass removed.
func (s Stats) Saved() int { return s.BytesBefore - s.BytesAfter }

// Compress rewrites the tool results of one upstream request body, in place at
// the JSON level so every member the gateway does not model survives. It
// returns the original bytes when nothing changed.
//
// The body shapes it understands are the ones a gateway sees: OpenAI chat
// messages (a tool message's content, string or parts), Anthropic messages (a
// tool_result block's content, string or parts), and Responses input items (a
// function_call_output's output, string or parts).
func Compress(body []byte, filters []string) ([]byte, Stats, error) {
	var stats Stats
	if len(body) == 0 {
		return body, stats, nil
	}
	envelope, ok := decodeObject(body)
	if !ok {
		return body, stats, fmt.Errorf("the request body is not a JSON object")
	}
	var member string
	switch {
	case envelope["messages"] != nil:
		member = "messages"
	case envelope["input"] != nil:
		member = "input"
	default:
		return body, stats, nil
	}

	items, ok := decodeArray(envelope[member])
	if !ok {
		return body, stats, nil
	}
	allowed := allowedSet(filters)
	changed := false
	for index, item := range items {
		next, itemChanged := compressItem(item, allowed, &stats)
		if itemChanged {
			items[index] = next
			changed = true
		}
	}
	if !changed {
		return body, stats, nil
	}
	encoded, err := marshalArray(items)
	if err != nil {
		return body, stats, fmt.Errorf("re-encoding the tool results: %w", err)
	}
	envelope[member] = encoded
	rewritten, err := marshalObject(envelope)
	if err != nil {
		return body, stats, fmt.Errorf("re-encoding the request body: %w", err)
	}
	return rewritten, stats, nil
}

// compressItem rewrites one message or input item, reporting whether it
// changed.
func compressItem(raw json.RawMessage, allowed func(string) bool, stats *Stats) (json.RawMessage, bool) {
	item, ok := decodeObject(raw)
	if !ok {
		return raw, false
	}
	var changed bool
	switch {
	case hasMemberValue(item, "type", "function_call_output"):
		changed = compressResponsesOutput(item, allowed, stats)
	case hasMemberValue(item, "role", "tool"):
		changed = compressToolContent(item, allowed, stats)
	default:
		changed = compressToolResults(item, allowed, stats)
	}
	if !changed {
		return raw, false
	}
	encoded, err := marshalObject(item)
	if err != nil {
		return raw, false
	}
	return encoded, true
}
