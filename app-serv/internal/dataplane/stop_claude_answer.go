// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stop_claude_answer.go
// @for       Honouring the caller's `stop_sequences` on a one-body Anthropic answer
//
//	the upstream answered past.
//
// @uses      encoding/json, internal/schema.
// @reason    The Anthropic wire makes the same promise the OpenAI wire does and
//
//	keeps it differently: a cut there is reported as
//	`stop_reason: "stop_sequence"` with the marker itself in
//	`stop_sequence`, not as a bare `stop`. Measured live on 2026-09-30 against
//	codebuddy-intl through /api/v1/messages, an upstream that ignores
//	`stop` answered `A STOPHERE B` in full and closed with `end_turn`, so
//	the caller received text it had asked not to see and a reason that said the
//	model had simply finished. This runs on the body the gateway serves, after
//	translation, so it holds whichever way the upstream wrote — translated from
//	OpenAI or Responses, or forwarded from a Claude one untouched.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-30
package dataplane

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// cutClaudeAnswer trims one served Anthropic answer to before the caller's first
// stop sequence, and returns it unchanged when the answer contains none.
//
// Only text blocks are cut and only text blocks after the marker are dropped. A
// `tool_use` block is not prose the caller asked to stop reading: it is work a
// client dispatches on, and deleting one because it happened to follow a marker
// would silently drop a call the caller must still make.
//
// The rewrite happens on the decoded object rather than the typed response for the
// reason stampAnswerModel gives: a body the gateway forwards untouched keeps every
// member the schema does not model.
func cutClaudeAnswer(body []byte, sequences []string) []byte {
	guard := newStopGuard(sequences)
	if guard == nil {
		return body
	}
	answer, ok := decodeObject(body)
	if !ok {
		return body
	}
	blocks, ok := arrayField(answer, "content")
	if !ok {
		return body
	}

	kept := make([]json.RawMessage, 0, len(blocks))
	cut := false
	for _, raw := range blocks {
		block, ok := decodeObject(raw)
		if !ok {
			kept = append(kept, raw)
			continue
		}
		if stringField(block, "type") != schema.BlockText {
			kept = append(kept, raw)
			continue
		}
		if cut {
			continue
		}
		text := stringField(block, "text")
		trimmed := guard.write(text)
		if trimmed == text && !guard.stopped() {
			kept = append(kept, raw)
			continue
		}
		cut = guard.stopped()
		if trimmed != "" {
			block["text"] = mustJSON(trimmed)
			kept = append(kept, mustJSON(block))
		}
	}
	if !cut {
		return body
	}
	answer["content"] = mustJSON(kept)
	answer["stop_reason"] = mustJSON(StopStopSequence)
	answer["stop_sequence"] = mustJSON(guard.sequence())
	encoded, err := json.Marshal(answer)
	if err != nil {
		// reason: every member came from a body that already decoded, and the three
		// this rewrites are a string, a string and an array of objects; a body that
		// survived the upstream is not failed over a label the gateway could not
		// re-encode.
		return body
	}
	return encoded
}
