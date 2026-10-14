// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_claude_tools.go
// @for       Buffering one stream's tool-call arguments so the input is corrected once, when the block closes.
// @uses      encoding/json, strings, internal/schema.
// @reason    A stream carries a tool's arguments in fragments, and an argument name is only knowable once the object that carries it is whole. Correcting `cmd` to `command` mid-fragment would mean rewriting text the model is still spelling, so the fragments are held and one corrected delta is emitted at the block's close.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// claudeToolBlocks holds one stream's open tool blocks by Anthropic block index.
type claudeToolBlocks struct {
	blocks map[int]*claudeToolBlock
}

// claudeToolBlock is one tool call: the name its arguments are being rewritten
// against, and the fragments gathered so far.
type claudeToolBlock struct {
	name string
	args strings.Builder
}

// newClaudeToolBlocks builds an empty buffer for one stream.
func newClaudeToolBlocks() *claudeToolBlocks {
	return &claudeToolBlocks{blocks: make(map[int]*claudeToolBlock, 4)}
}

// open starts a block's buffer. Arguments arriving before a block was opened are
// dropped, which is what an unknown index means.
func (t *claudeToolBlocks) open(index int, name string) {
	t.blocks[index] = &claudeToolBlock{name: name}
}

// append gathers one argument fragment.
func (t *claudeToolBlocks) append(index int, fragment string) {
	if block, ok := t.blocks[index]; ok {
		block.args.WriteString(fragment)
	}
}

// flushFrame renders the single input_json_delta that carries a block's whole
// input, corrected onto the names the tool declares. A block that gathered
// nothing gets no frame: an empty tool input is the client's own reading of `{}`
// from the block start, and a delta invented here would say the model named
// parameters it never sent.
func (t *claudeToolBlocks) flushFrame(index int) []byte {
	block, ok := t.blocks[index]
	if !ok || block.args.Len() == 0 {
		return nil
	}
	input := normalizeClaudeToolInput(block.name, json.RawMessage(block.args.String()))
	return eventFrame(schema.EventContentBlockDelta, mustFrame(map[string]any{
		"type":  schema.EventContentBlockDelta,
		"index": index,
		"delta": map[string]any{"type": schema.DeltaInputJSON, "partial_json": string(input)},
	}))
}
