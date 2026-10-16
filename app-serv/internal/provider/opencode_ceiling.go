// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_ceiling.go
// @for       The output ceiling an OpenCode request may carry, on either wire: the floor the model needs, the cap the wire takes, and the bound a raise may not pass.
// @uses      encoding/json, strconv, internal/registry.
// @reason    The upstream refuses a ceiling below sixteen and answers 400 once the ceiling plus the input passes the model's context, and a model that thinks by default spends a small ceiling entirely on reasoning and returns an answer-less body the upstream counts as success (measured live 2026-10-16: max_tokens 2500, 2499 of it thinking, 0 content). The Responses wire has bounded ceilings since September; the chat wire forwarded the client's number untouched, which is why one wire behaved and the other starved. Held apart from opencode_body.go because the request shape and the arithmetic of one bound are two concerns, and the file was past the AGENTS.md §1.1 comfort line with both in it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-16
package provider

import (
	"encoding/json"
	"strconv"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// openCodeMinOutputTokens is the floor the OpenCode Console API accepts for an
// output ceiling. A lower value is refused with
// "max_output_tokens The number must be >= 16" (measured live, 2026-09-26),
// which is what the combo test's one-token ping used to send.
const openCodeMinOutputTokens = 16

// openCodeMaxOutputTokens is the ceiling the wire takes for one answer. Asking
// for more is answered with 400 "The request contains invalid parameters" once
// the input plus the ceiling passes the model's context window, which a
// 1M-context combo hits the moment a client asks for the whole window.
const openCodeMaxOutputTokens = 65536

// openCodeBlindRaiseBound is the largest ceiling the gateway will RAISE a request
// to when the model declares no context window. Cutting an oversized ceiling is
// safe and the wire's own rule; writing a bigger number than the client sent is
// not, because a raise is the one place this package guesses at the window.
const openCodeBlindRaiseBound = 4096

// renameOpenCodeCeiling moves a chat-shaped output ceiling onto the Responses
// field name. The Responses wire answers 400 to `max_tokens`, and an already
// present `max_output_tokens` wins because it is the client's own explicit
// choice for this wire. The renamed value is then raised to the floor this
// request's model declares, because a ceiling below it is refused by the
// upstream rather than rounded by it, or answered with nothing at all.
func renameOpenCodeCeiling(body map[string]json.RawMessage, model registry.Model) {
	floor := openCodeOutputFloor(model)
	if _, present := body["max_output_tokens"]; present {
		delete(body, "max_tokens")
		delete(body, "max_completion_tokens")
		clampOpenCodeCeiling(body, floor)
		return
	}
	for _, key := range []string{"max_completion_tokens", "max_tokens"} {
		if value, present := body[key]; present {
			body["max_output_tokens"] = value
			break
		}
	}
	delete(body, "max_tokens")
	delete(body, "max_completion_tokens")
	clampOpenCodeCeiling(body, floor)
}

// clampOpenCodeChatCeiling bounds the ceiling the chat wire reads, under the same
// floor and cap the Responses wire enforces. It never invents a ceiling the client
// did not send and never rewrites one that is not an integer: the raise is bounded
// by openCodeChatRaiseBound, and a floor that cannot be raised to within that bound
// leaves the client's own number standing, because a thin answer is still an answer
// while a 400 is a failed request.
func clampOpenCodeChatCeiling(body map[string]json.RawMessage, model registry.Model) {
	key, found := openCodeChatCeilingKey(body)
	if !found {
		return
	}
	var value int
	if err := json.Unmarshal(body[key], &value); err != nil {
		return
	}
	bounded := value
	floor := openCodeOutputFloor(model)
	if bounded < floor && floor <= openCodeChatRaiseBound(model) {
		bounded = floor
	}
	if bounded > openCodeMaxOutputTokens {
		bounded = openCodeMaxOutputTokens
	}
	if bounded != value {
		body[key] = json.RawMessage(strconv.Itoa(bounded))
	}
}

// openCodeChatCeilingKey names the ceiling the chat upstream reads when a client
// sent both spellings, mirroring the precedence renameOpenCodeCeiling follows.
func openCodeChatCeilingKey(body map[string]json.RawMessage) (string, bool) {
	for _, key := range []string{"max_completion_tokens", "max_tokens"} {
		if _, present := body[key]; present {
			return key, true
		}
	}
	return "", false
}

// openCodeChatRaiseBound is how far a raise may reach: half the model's declared
// window, cut by its declared maximum output, and stopped at the blind bound when
// the model declares no window at all.
func openCodeChatRaiseBound(model registry.Model) int {
	bound := openCodeBlindRaiseBound
	if model.ContextLength > 0 {
		bound = min(bound0(model.ContextLength/2), openCodeMaxOutputTokens)
	}
	if model.MaxOutputTokens > 0 {
		bound = min(bound, model.MaxOutputTokens)
	}
	return bound
}

// bound0 keeps a declared window of one or two from producing a zero bound, which
// would silently disable every raise.
func bound0(half int) int {
	if half < openCodeMinOutputTokens {
		return openCodeMinOutputTokens
	}
	return half
}

// openCodeOutputFloor is the ceiling the smallest accepted request must carry:
// the wire's own floor, raised to whatever the model declares it needs to answer
// at all. muse-spark-1.3-contributor-free spends a small ceiling entirely on
// reasoning and returns an empty body with finish_reason "length" (measured live,
// 2026-09-28), which the upstream counts as a successful answer. A floor is a
// mitigation, not a cure: measured 2026-10-16, one request spent 2499 of its 2500
// token ceiling on thinking, so any ceiling can be consumed whole.
func openCodeOutputFloor(model registry.Model) int {
	if model.MinOutputTokens > openCodeMinOutputTokens {
		return model.MinOutputTokens
	}
	return openCodeMinOutputTokens
}

// clampOpenCodeCeiling bounds an integer ceiling between the floor this
// request's model needs and the largest answer the wire will take. A ceiling the
// client never sent stays absent, raising a value that exists is the narrow
// licence, inventing one the client did not ask for is not. A member that is not
// a JSON integer is left exactly as the client wrote it: the upstream's own
// validation answers for it, and rewriting a malformed value would hide the
// client's bug instead of reporting it.
//
// The ceiling is applied last, so a model declaring a floor above it resolves to
// the ceiling, the same order the reference's clamp reads.
func clampOpenCodeCeiling(body map[string]json.RawMessage, floor int) {
	raw, present := body["max_output_tokens"]
	if !present {
		return
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return
	}
	bounded := value
	if bounded < floor {
		bounded = floor
	}
	if bounded > openCodeMaxOutputTokens {
		bounded = openCodeMaxOutputTokens
	}
	if bounded != value {
		body["max_output_tokens"] = json.RawMessage(strconv.Itoa(bounded))
	}
}
