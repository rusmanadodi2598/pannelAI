// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/answer_toolnames_chat.go
// @for       The chat wire's half of the tool-name restore: the tool_calls arrays, and the accumulator that rejoins a name the upstream split across chunks.
// @uses      strings.
// @reason    A streamed chat delta may carry a tool call's name in fragments (the upstream writes `read_client` as `read_cli` then `ent`), so the fragments are rejoined before the name can be matched. The accumulator hands the client the longest part it can already justify and holds the rest back until the call's next chunk proves the name complete, because a fragment that merely starts an alias could still turn out to be a caller name of its own. The Responses and Anthropic wires name a call whole and live in answer_toolnames.go.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"strings"
)

// toolCallKey identifies one tool call across a stream's chunks: the choice it
// was reported under and the index the wire gives it.
type toolCallKey struct {
	choice int
	index  int
}

// toolNameAccumulator rejoins one call's streamed name fragments.
type toolNameAccumulator struct {
	// seen is every name fragment the upstream has sent for this call.
	seen string
	// emitted is the length of the restored name the client already has, so
	// the next fragment hands over only the part still missing.
	emitted int
	// closed reports that the call's name stopped growing, so a later
	// fragment opens a fresh name rather than extending this one.
	closed bool
}

// rewriteChoices restores the names in the chat wire's tool_calls arrays: a
// streamed delta, whose name may arrive in fragments, and the complete message
// a non-streamed answer carries.
func (r *toolNameRewriter) rewriteChoices(chunk object) bool {
	choices, ok := arrayField(chunk, "choices")
	if !ok {
		return false
	}
	changed := false
	for choiceIndex, raw := range choices {
		choice, ok := decodeObject(raw)
		if !ok {
			continue
		}
		choiceChanged := false
		if delta, ok := objectField(choice, "delta"); ok {
			if r.rewriteToolCalls(delta, choiceIndex, true) {
				choice["delta"] = mustJSON(delta)
				choiceChanged = true
			}
		}
		if message, ok := objectField(choice, "message"); ok {
			if r.rewriteToolCalls(message, choiceIndex, false) {
				choice["message"] = mustJSON(message)
				choiceChanged = true
			}
		}
		if choiceChanged {
			choices[choiceIndex] = mustJSON(choice)
			changed = true
		}
	}
	if changed {
		chunk["choices"] = mustJSON(choices)
	}
	return changed
}

// rewriteToolCalls restores the names in one tool_calls array. A streamed
// entry carries fragments and is accumulated per call index; a non-streamed
// one carries the complete name and is matched whole.
func (r *toolNameRewriter) rewriteToolCalls(holder object, choiceIndex int, streamed bool) bool {
	calls, ok := arrayField(holder, "tool_calls")
	if !ok {
		return false
	}
	changed := false
	for entryIndex, raw := range calls {
		call, ok := decodeObject(raw)
		if !ok {
			continue
		}
		function, ok := objectField(call, "function")
		if !ok {
			continue
		}
		key := toolCallKey{choice: choiceIndex, index: entryIndex}
		if streamed {
			key.index = intField(call, "index")
		}
		if !r.rewriteFunctionName(function, key, streamed) {
			continue
		}
		call["function"] = mustJSON(function)
		calls[entryIndex] = mustJSON(call)
		changed = true
	}
	if changed {
		holder["tool_calls"] = mustJSON(calls)
	}
	return changed
}

// rewriteFunctionName restores one tool call's function name, reporting
// whether the function object changed.
func (r *toolNameRewriter) rewriteFunctionName(function object, key toolCallKey, streamed bool) bool {
	name := stringField(function, "name")
	if !streamed {
		return name != "" && r.restoreName(function)
	}
	changed := false
	if name != "" {
		changed = r.accumulateName(key, function, name)
	}
	// A fragment that carries arguments, or no name at all, has stopped
	// growing the name: the call is closed, and any held-back suffix is handed
	// over with it.
	if _, arguments := function["arguments"]; name == "" || arguments {
		if r.closeName(key, function) {
			changed = true
		}
	}
	return changed
}

// accumulateName rejoins one name fragment and hands the client the longest
// part of the restored name it can already justify. It reports whether this
// chunk's own bytes changed, so a fragment that is already what the caller
// should read keeps the payload it arrived in.
func (r *toolNameRewriter) accumulateName(key toolCallKey, function object, fragment string) bool {
	acc, ok := r.calls[key]
	if !ok || acc.closed {
		acc = &toolNameAccumulator{}
		r.calls[key] = acc
	}
	acc.seen += fragment
	restored := r.restoredPrefix(acc.seen)
	delta := restored[acc.emitted:]
	acc.emitted = len(restored)
	if delta == fragment {
		return false
	}
	function["name"] = mustJSON(delta)
	return true
}

// closeName marks one call's name complete and hands over whatever the prefix
// rule held back. The suffix is appended to the fragment this same chunk may
// carry, so a name that arrived together with its first arguments is not left
// short.
func (r *toolNameRewriter) closeName(key toolCallKey, function object) bool {
	acc, ok := r.calls[key]
	if !ok || acc.closed {
		return false
	}
	acc.closed = true
	tail := acc.restoredTail(r.aliases)
	if tail == "" {
		return false
	}
	function["name"] = mustJSON(stringField(function, "name") + tail)
	return true
}

// restoredPrefix returns the longest prefix of the caller's name the fragments
// so far prove: while the text is still a prefix of an alias, only that alias's
// own name can be restored, and the part that would belong to the alias's
// suffix is held back; once the text can no longer become an alias it stands as
// written.
//
// The alias scheme builds every alias as the caller's name plus a suffix, so
// aliases that share a prefix share the original name too: the candidates
// cannot disagree.
func (r *toolNameRewriter) restoredPrefix(seen string) string {
	for alias, original := range r.aliases {
		if !strings.HasPrefix(alias, seen) {
			continue
		}
		if len(seen) > len(original) {
			return original
		}
		return seen
	}
	return seen
}

// restoredTail returns the suffix a closed name still owes the client: the
// complete name (the alias's original, or the text itself when it was never an
// alias) minus what was already handed over.
func (acc *toolNameAccumulator) restoredTail(aliases map[string]string) string {
	full, found := aliases[acc.seen]
	if !found {
		full = acc.seen
	}
	if len(full) <= acc.emitted {
		return ""
	}
	return full[acc.emitted:]
}
