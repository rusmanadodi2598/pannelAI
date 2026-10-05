// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_stream_openai_sanitize.go
// @for       Shaping one forwarded OpenAI chunk before the client sees it: the
//
//	empty members a vendor puts on every frame, and the cut at the
//	caller's stop sequences.
//
// @uses      encoding/json, internal/dataplane object.
// @reason    A same-format stream forwards the upstream's frames so that no
//
//	unmodelled field is lost, but that promise also carries through what
//	a vendor should never have sent. Measured live on
//	codebuddy-intl/deepseek-v4.1-flash (2026-09-30): every delta arrived
//	with `tool_calls: []`, `function_call: null`, `refusal: ""` and
//	`extra_fields: null`, and the finish reason arrived as `""` rather
//	than null. A client that tests `if delta.tool_calls` reads a truthy
//	empty list on all eleven frames of a five-token answer, and a client
//	switching on `finish_reason` sees a value that is neither null nor a
//	reason. The same measurement showed `stop` ignored outright, so the
//	cut is applied here too. Both work per frame and remove nothing that
//	carries a value.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-30
package dataplane

import (
	"bytes"
	"encoding/json"
)

// emptyDeltaMembers are the delta members that say nothing when they arrive empty
// and that a client therefore reads wrong: a deprecated call shape, a refusal that
// refused nothing, a thinking fragment of zero length. `content` is deliberately
// absent, because an empty content string is what the role-opening frame carries.
var emptyDeltaMembers = []string{"function_call", "refusal", "extra_fields", "reasoning_content"}

// sanitizeChunk rewrites one forwarded chunk into what the client should read, and
// reports whether the chunk is worth a frame at all.
//
// It runs after the identity members are rewritten and before the frame is
// encoded, so the vendor's own finish frame and a cut the gateway performs are
// decided in the same pass, in the order the client sees them.
func (s *StreamState) sanitizeChunk(chunk object) bool {
	choices, ok := arrayField(chunk, "choices")
	if !ok || len(choices) == 0 {
		return true
	}
	changed := false
	for index, raw := range choices {
		choice, ok := decodeObject(raw)
		if !ok {
			continue
		}
		pruned := pruneChoice(choice)
		cut := s.cutChoice(choice)
		if pruned || cut {
			changed = true
			choices[index] = mustJSON(choice)
		}
		// Only a frame the cut emptied is withheld. A close the upstream sent on
		// its own keeps its place on the wire even when its reason was nulled as a
		// duplicate, because the members still on it, a trailing role delta, the
		// usage, are the client's to read.
		if cut && !frameCarriesAnswer(choice) && !carriesUsage(chunk) {
			return false
		}
	}
	if changed {
		chunk["choices"] = mustJSON(choices)
	}
	return true
}

// pruneChoice drops the empty members a vendor wrote onto a choice and reports
// whether it changed anything.
func pruneChoice(choice object) bool {
	// An empty finish reason is not a reason: OpenAI sends null between frames,
	// and a client that compares the member to null or switches on it as an enum
	// is handed a third answer by an empty string.
	changed := false
	if _, present := choice["finish_reason"]; present && stringField(choice, "finish_reason") == "" {
		choice["finish_reason"] = mustJSON(nil)
		changed = true
	}
	delta, ok := objectField(choice, "delta")
	if !ok {
		return changed
	}
	for _, member := range emptyDeltaMembers {
		if carriesNoValue(delta[member]) {
			delete(delta, member)
			changed = true
		}
	}
	if carriesNoValue(delta["tool_calls"]) {
		delete(delta, "tool_calls")
		changed = true
	}
	if changed {
		choice["delta"] = mustJSON(delta)
	}
	return changed
}

// cutChoice trims a frame's content to before the caller's stop sequence and closes
// the stream there. It reports whether it changed the frame.
//
// Only a frame carrying text can be cut. Once the marker has passed, the guard
// returns an empty string for every later fragment, which is the answer's text
// being withdrawn rather than the client's own text being withheld.
func (s *StreamState) cutChoice(choice object) bool {
	if s.stop == nil {
		return false
	}
	delta, ok := objectField(choice, "delta")
	if !ok {
		return false
	}
	content := stringField(delta, "content")
	if content == "" {
		return false
	}
	kept := s.stop.write(content)
	if kept == content && !s.stop.stopped() {
		return false
	}
	if kept == "" {
		delete(delta, "content")
	} else {
		delta["content"] = mustJSON(kept)
	}
	choice["delta"] = mustJSON(delta)
	// The cut is announced once, on the frame that carried the marker: a later
	// frame re-reporting it would hand the client a second close.
	if s.stop.stopped() && !s.cutAnnounced {
		s.cutAnnounced = true
		// The caller's marker is where this answer ended, whatever the upstream
		// went on to report after it.
		choice["finish_reason"] = mustJSON(FinishStop)
		s.finishReason = FinishStop
		s.finishSent = true
	}
	return true
}

// frameCarriesAnswer reports whether a choice still says anything now that its
// empty members are gone. A delta emptied by the cut, with no finish reason left to
// report, is a frame the client gains nothing from reading.
func frameCarriesAnswer(choice object) bool {
	if stringField(choice, "finish_reason") != "" {
		return true
	}
	delta, ok := objectField(choice, "delta")
	if !ok {
		return true
	}
	for _, member := range []string{"content", "reasoning_content", "role", "tool_calls", "function_call"} {
		if !carriesNoValue(delta[member]) {
			return true
		}
	}
	return false
}

// carriesUsage reports whether this frame is the delivery of the call's numbers.
//
// A frame emptied by the pruning still has to reach the client when it carries
// them: a vendor that states usage on its closing frame, and then again on a frame
// after it, leaves the gateway nulling the duplicate reason, and dropping that
// frame for having no reason left would drop the usage with it.
func carriesUsage(chunk object) bool {
	raw, present := chunk["usage"]
	if !present {
		return false
	}
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// carriesNoValue reports whether a raw member is absent or says nothing: null, the
// empty string, an empty array, or an object whose every member says nothing.
//
// The object rule is what catches this vendor's deprecated `function_call`, which
// arrives as `{"name":"","arguments":""}`, a block that is neither empty nor
// informative, and one a client reading the deprecated field takes for a call.
func carriesNoValue(raw json.RawMessage) bool {
	return memberCarriesNoValue(raw, 2)
}

func memberCarriesNoValue(raw json.RawMessage, depth int) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return true
	}
	switch trimmed[0] {
	case '{':
		if depth == 0 {
			return false
		}
		nested, ok := decodeObject(trimmed)
		if !ok {
			return false
		}
		if len(nested) == 0 {
			return true
		}
		for _, value := range nested {
			if !memberCarriesNoValue(value, depth-1) {
				return false
			}
		}
		return true
	case '[':
		var entries []json.RawMessage
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return false
		}
		return len(entries) == 0
	}
	return string(trimmed) == "null" || string(trimmed) == `""`
}
