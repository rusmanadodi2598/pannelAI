// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_responses_pairing.go
// @for       The call/output pairing the OpenCode Responses wire insists a conversation carries.
// @uses      encoding/json.
// @reason    /inference/v1/responses rejects a `function_call` with no matching `function_call_output` with 400 "No tool output found for function call ...", and a compacted or interrupted Claude Code turn leaves exactly that behind in the history it replays. An answer the client never got an answer to still has to be closed, so the pairing is completed here rather than dropped at the upstream.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-14
package provider

import "encoding/json"

// openCodeItemKind names the two input items whose pairing the wire checks.
type openCodeItemKind uint8

const (
	openCodeItemOther openCodeItemKind = iota
	openCodeItemCall
	openCodeItemCallOutput
)

// openCodeItem is one corrected input item. callID is the value the wire will
// actually see, after the clamp, because pairing that agreed with the
// un-clamped value would disagree with the upstream about which calls are open.
type openCodeItem struct {
	raw    json.RawMessage
	kind   openCodeItemKind
	callID string
}

// pairOpenCodeItems drops outputs no call asked for and closes calls no output
// answered, in the item order the client sent. Presence is decided across the
// whole conversation, not between neighbours: a call whose output arrives
// twenty items later is paired, which is what the reference's own test does.
func pairOpenCodeItems(items []openCodeItem) []json.RawMessage {
	calls := make(map[string]int, len(items))
	outputs := make(map[string]int, len(items))
	for _, item := range items {
		switch item.kind {
		case openCodeItemCall:
			calls[item.callID]++
		case openCodeItemCallOutput:
			outputs[item.callID]++
		}
	}

	paired := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		switch item.kind {
		case openCodeItemCall:
			paired = append(paired, item.raw)
			if outputs[item.callID] == 0 {
				paired = append(paired, openCodeSyntheticOutput(item.callID))
				// A second call on the same id is closed by the same answer;
				// the reference adds the id to one set and reads that set.
				outputs[item.callID]++
			}
		case openCodeItemCallOutput:
			if calls[item.callID] == 0 {
				continue
			}
			paired = append(paired, item.raw)
		default:
			paired = append(paired, item.raw)
		}
	}
	return paired
}

// openCodeSyntheticOutput closes a call with the empty object the reference
// writes: an answer the model never gave, recorded as a result the client can
// dispatch rather than a hole the upstream refuses to read.
func openCodeSyntheticOutput(callID string) json.RawMessage {
	return mustRaw(map[string]json.RawMessage{
		"type":    mustRawString("function_call_output"),
		"call_id": mustRawString(callID),
		"output":  mustRawString("{}"),
	})
}
