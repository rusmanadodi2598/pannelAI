// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/answer_identity.go
// @for       The model name one answer is served under, and the rewrite that places it on a body the gateway would otherwise forward untouched.
// @uses      encoding/json, internal/dataplane Resolution.
// @reason    A combo is a name an operator wrote and a client sends, so the answer it produces has to carry that name back: the resolved member is a routing fact the caller cannot re-send, and a round_robin combo hands the same client a different member on every request. SPEC-API-001 §7.6 fixes the rule. The same-wire passthrough is the one path that never reaches a translation where a name could be chosen, so the stamp lives here.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package dataplane

import "encoding/json"

// ClientModel reports the name the client addressed: the combo a model string
// resolved through, otherwise the model string itself. It is a method rather than
// a stored field because resolveCombo already overwrites Combo with the combo the
// client addressed, so this answer has one source instead of two to keep in step.
// The direct-model half needs the raw string: ModelID has already lost the
// provider prefix and UpstreamID carries the vendor's own alias, so only the
// string the caller sent round-trips back to it.
func (r Resolution) ClientModel() string {
	if r.IsCombo() {
		return r.Combo.Name()
	}
	return r.Requested
}

// answerModel is the model name one answer carries: the name the client
// addressed, a combo name or the model string it sent, and only failing that the
// name the calling surface already used. The fallback stays with the caller rather
// than being fixed here because a Resolution built outside the relay, a test, or a
// seam that resolves one member on its own carries no addressed string at all, and
// such a surface knows which of its own identifiers stands in.
func answerModel(r Resolution, fallback string) string {
	if name := r.ClientModel(); name != "" {
		return name
	}
	return fallback
}

// stampAnswerModel writes model onto a raw answer the gateway forwards untouched,
// leaving every other member exactly as the upstream wrote it. Re-encoding the
// decoded object rather than a typed response is what keeps unmodelled fields
// alive, and the stream re-framer rewrites its identity the same way. An empty
// name, an unchanged name, or a body that does not decode leaves the bytes alone: a
// call that reached the upstream and came back with 200 is not failed over a label
// the gateway could not read.
func stampAnswerModel(raw []byte, model string) []byte {
	if model == "" {
		return raw
	}
	body, ok := decodeObject(raw)
	if !ok {
		return raw
	}
	if stringField(body, "model") == model {
		return raw
	}
	body["model"] = mustJSON(model)
	encoded, err := json.Marshal(body)
	if err != nil {
		return raw
	}
	return encoded
}
