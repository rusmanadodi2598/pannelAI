// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/answer_toolnames.go
// @for       Restoring the caller's own tool names in an upstream answer before it is translated.
// @uses      internal/schema, encoding/json.
// @reason    A connector may rename a caller tool to get the body past its upstream (OpenCode's fingerprint pass conceals a caller `read` as `read_client`), and the upstream then answers naming the alias; the map travels from the shaped request to the answer through Upstream.ToolAliases. The restore runs on the upstream's own wire before translation, so one pass covers all three client wires, and a body with nothing aliased is returned as the same bytes it arrived in. The chat wire's split-name accumulator lives in answer_toolnames_chat.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// restoreToolNames rewrites the tool names one upstream answer echoes back to
// the names the caller declared. It is the non-streamed half of the seam: the
// whole body is restored at once, in whichever wire the upstream wrote it.
func restoreToolNames(raw []byte, aliases map[string]string) []byte {
	rewriter := newToolNameRewriter(aliases)
	if rewriter == nil {
		return raw
	}
	return rewriter.rewrite(raw)
}

// restoreToolNamesStream wraps a stream translator so every upstream payload
// is restored before it is translated. The rewriter is per-stream state: the
// chat wire may split one tool name across chunks, and rejoining it is what
// answer_toolnames_chat.go accumulates.
func restoreToolNamesStream(aliases map[string]string, translate func(string, []byte) [][]byte) func(string, []byte) [][]byte {
	rewriter := newToolNameRewriter(aliases)
	if rewriter == nil {
		return translate
	}
	return func(upstreamTarget string, payload []byte) [][]byte {
		return translate(upstreamTarget, rewriter.rewrite(payload))
	}
}

// toolNameRewriter restores the caller's tool names in one answer, or one
// stream. It is owned by a single request and never shared.
type toolNameRewriter struct {
	// aliases maps a name the request sent upstream back to the name the
	// caller declared. A live rewriter always has at least one.
	aliases map[string]string
	// calls accumulates the chat wire's streamed names by the call they
	// belong to, because that wire may split one name across chunks.
	calls map[toolCallKey]*toolNameAccumulator
}

// newToolNameRewriter builds the rewriter for one answer, or nil when the
// request renamed nothing, which is what keeps an untouched answer untouched.
func newToolNameRewriter(aliases map[string]string) *toolNameRewriter {
	if len(aliases) == 0 {
		return nil
	}
	return &toolNameRewriter{aliases: aliases, calls: make(map[toolCallKey]*toolNameAccumulator, 4)}
}

// rewrite restores one upstream payload, in whichever wire shape it arrived.
// The original bytes are returned when nothing matched, so a payload with no
// aliased name is forwarded byte-identically.
func (r *toolNameRewriter) rewrite(payload []byte) []byte {
	chunk, ok := decodeObject(payload)
	if !ok {
		return payload
	}
	changed := r.rewriteChoices(chunk)
	if r.rewriteResponsesItems(chunk) {
		changed = true
	}
	if r.rewriteClaudeBlocks(chunk) {
		changed = true
	}
	if !changed {
		return payload
	}
	encoded, err := json.Marshal(chunk)
	if err != nil {
		// reason: the payload decoded once already, so a re-encode failure
		// means a member holds a value json cannot render; forwarding the
		// original bytes is closer to correct than dropping the payload.
		return payload
	}
	return encoded
}

// rewriteResponsesItems restores the names the Responses wire echoes in its
// function_call items: one item on a streamed event, the whole output list on
// a non-streamed body, and the response object a terminal event carries.
func (r *toolNameRewriter) rewriteResponsesItems(chunk object) bool {
	changed := false
	if item, ok := objectField(chunk, "item"); ok && r.restoreItemName(item) {
		chunk["item"] = mustJSON(item)
		changed = true
	}
	if response, ok := objectField(chunk, "response"); ok && r.rewriteOutputItems(response) {
		chunk["response"] = mustJSON(response)
		changed = true
	}
	if r.rewriteOutputItems(chunk) {
		changed = true
	}
	return changed
}

// rewriteOutputItems restores every call item in one output array.
func (r *toolNameRewriter) rewriteOutputItems(holder object) bool {
	items, ok := arrayField(holder, "output")
	if !ok {
		return false
	}
	changed := false
	for index, raw := range items {
		item, ok := decodeObject(raw)
		if !ok || !r.restoreItemName(item) {
			continue
		}
		items[index] = mustJSON(item)
		changed = true
	}
	if !changed {
		return false
	}
	holder["output"] = mustJSON(items)
	return true
}

// restoreItemName restores one Responses call item's name, in either the
// standard or the custom-tool spelling the stream mapper reads the same.
func (r *toolNameRewriter) restoreItemName(item object) bool {
	if !isResponsesCallItem(item) {
		return false
	}
	return r.restoreName(item)
}

// rewriteClaudeBlocks restores the names in the Anthropic wire's tool_use
// blocks: the content_block_start event that opens a streamed block, and the
// content array a non-streamed message carries.
func (r *toolNameRewriter) rewriteClaudeBlocks(chunk object) bool {
	changed := false
	if block, ok := objectField(chunk, "content_block"); ok &&
		stringField(block, "type") == schema.BlockToolUse && r.restoreName(block) {
		chunk["content_block"] = mustJSON(block)
		changed = true
	}
	blocks, ok := arrayField(chunk, "content")
	if !ok {
		return changed
	}
	blocksChanged := false
	for index, raw := range blocks {
		block, ok := decodeObject(raw)
		if !ok || stringField(block, "type") != schema.BlockToolUse || !r.restoreName(block) {
			continue
		}
		blocks[index] = mustJSON(block)
		blocksChanged = true
	}
	if blocksChanged {
		chunk["content"] = mustJSON(blocks)
		changed = true
	}
	return changed
}

// restoreName swaps one holder's tool name for the caller's, reporting whether
// it changed. A name the caller never declared is left exactly as it arrived.
func (r *toolNameRewriter) restoreName(holder object) bool {
	original, found := r.aliases[stringField(holder, "name")]
	if !found {
		return false
	}
	holder["name"] = mustJSON(original)
	return true
}
