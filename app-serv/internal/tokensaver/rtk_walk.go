// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/rtk_walk.go
// @for       The four wire shapes RTK rewrites, the text decision, and the
//
//	JSON helpers that keep an untouched member byte for byte.
//
// @uses      bytes, encoding/json.
// @reason    SPEC-API-002 §3 fixes which shapes carry a tool result: an OpenAI
//
//	tool message, an Anthropic tool_result block, and a Responses
//	function_call_output. Keeping them together is what makes the
//	"never touch a user message" rule readable in one screen.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"encoding/json"
)

// compressResponsesOutput rewrites a Responses function_call_output item, whose
// output is either a string or an array of input_text parts.
func compressResponsesOutput(item object, allowed func(string) bool, stats *Stats) bool {
	if _, ok := item.memberString("output"); ok {
		return compressMember(item, "output", "openai-responses-string", allowed, stats)
	}
	parts, ok := decodeArray(item["output"])
	if !ok {
		return false
	}
	next, changed := compressPartTexts(parts, "input_text", "openai-responses-array", allowed, stats)
	if !changed {
		return false
	}
	encoded, err := marshalArray(next)
	if err != nil {
		return false
	}
	item["output"] = encoded
	return true
}

// compressToolContent rewrites an OpenAI tool message, whose content is either a
// string or an array of text parts.
func compressToolContent(item object, allowed func(string) bool, stats *Stats) bool {
	if _, ok := item.memberString("content"); ok {
		return compressMember(item, "content", "openai-tool", allowed, stats)
	}
	parts, ok := decodeArray(item["content"])
	if !ok {
		return false
	}
	next, changed := compressPartTexts(parts, "text", "openai-tool-array", allowed, stats)
	if !changed {
		return false
	}
	encoded, err := marshalArray(next)
	if err != nil {
		return false
	}
	item["content"] = encoded
	return true
}

// compressToolResults rewrites the tool_result blocks of an Anthropic message.
// A block that reports an error is left alone: the model has to see the failure
// verbatim, which is the one place a compressed trace would be a lie.
func compressToolResults(item object, allowed func(string) bool, stats *Stats) bool {
	blocks, ok := decodeArray(item["content"])
	if !ok {
		return false
	}
	changed := false
	for index, raw := range blocks {
		block, ok := decodeObject(raw)
		if !ok || !hasMemberValue(block, "type", "tool_result") || block.memberTrue("is_error") {
			continue
		}
		if compressToolResultBlock(block, allowed, stats) {
			encoded, err := marshalObject(block)
			if err != nil {
				continue
			}
			blocks[index] = encoded
			changed = true
		}
	}
	if !changed {
		return false
	}
	encoded, err := marshalArray(blocks)
	if err != nil {
		return false
	}
	item["content"] = encoded
	return true
}

// compressToolResultBlock rewrites one tool_result block's content, string or
// parts.
func compressToolResultBlock(block object, allowed func(string) bool, stats *Stats) bool {
	if _, ok := block.memberString("content"); ok {
		return compressMember(block, "content", "claude-string", allowed, stats)
	}
	parts, ok := decodeArray(block["content"])
	if !ok {
		return false
	}
	next, changed := compressPartTexts(parts, "text", "claude-array", allowed, stats)
	if !changed {
		return false
	}
	encoded, err := marshalArray(next)
	if err != nil {
		return false
	}
	block["content"] = encoded
	return true
}

// compressPartTexts rewrites the text member of every part whose type matches.
func compressPartTexts(parts []json.RawMessage, partType, shape string, allowed func(string) bool, stats *Stats) ([]json.RawMessage, bool) {
	changed := false
	for index, raw := range parts {
		part, ok := decodeObject(raw)
		if !ok || !hasMemberValue(part, "type", partType) {
			continue
		}
		if !compressMember(part, "text", shape, allowed, stats) {
			continue
		}
		encoded, err := marshalObject(part)
		if err != nil {
			continue
		}
		parts[index] = encoded
		changed = true
	}
	return parts, changed
}

// compressMember rewrites one string member of an object, reporting whether it
// changed.
func compressMember(obj object, member, shape string, allowed func(string) bool, stats *Stats) bool {
	text, ok := obj.memberString(member)
	if !ok {
		return false
	}
	next := compressText(text, shape, allowed, stats)
	if next == text {
		return false
	}
	encoded, err := marshalString(next)
	if err != nil {
		return false
	}
	obj[member] = encoded
	return true
}

// compressText decides whether one blob is worth rewriting, and rewrites it with
// the filter autodetection picks from the allowed set.
//
// The floor, the cap, and the never-empty and never-grow rules are the
// reference's (SPEC-API-002 §5). Sizes are byte counts, which is what the
// reference's constants mean even though its own arithmetic counts UTF-16 units.
func compressText(text, shape string, allowed func(string) bool, stats *Stats) string {
	before := len(text)
	stats.BytesBefore += before
	if before < MinCompressSize || before > RawCap {
		stats.BytesAfter += before
		return text
	}
	fn, name, ok := detect(text, allowed)
	if !ok {
		stats.BytesAfter += before
		return text
	}
	out := safeApply(fn, text)
	if out == "" || len(out) >= before {
		stats.BytesAfter += before
		return text
	}
	stats.BytesAfter += len(out)
	stats.Hits = append(stats.Hits, Hit{Shape: shape, Filter: name, Saved: before - len(out)})
	return out
}

// safeApply runs one filter, treating a panic as "the filter declined": a
// malformed blob must never fail a request, which is the reference's
// catch_unwind equivalent.
func safeApply(fn filter, text string) (out string) {
	defer func() {
		if recover() != nil {
			out = text
		}
	}()
	return fn(text)
}
