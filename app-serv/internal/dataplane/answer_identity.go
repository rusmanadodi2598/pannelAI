// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/answer_identity.go
// @for       The model name one answer is served under, and the rewrite that
//
//	places it on a body the gateway would otherwise forward untouched.
//
// @uses      encoding/json, internal/dataplane Resolution.
// @reason    A combo is a name an operator wrote and a client sends, so the
//
//	answer it produces has to carry that name back: the resolved member is a
//	routing fact the caller cannot re-send, and a round_robin combo hands the
//	same client a different member on every request. SPEC-API-001 §7.6 fixes
//	the rule. The same-wire passthrough is the one path that never reaches a
//	translation where a name could be chosen, so the stamp lives here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-29
package dataplane

import "encoding/json"

// ClientModel reports the name the client addressed: the combo a model string
// resolved through, and "" when the request addressed a model directly.
//
// It is a method rather than a stored field because it is derived from the combo
// the resolution already carries: a second field holding the same answer is a
// second thing to keep in step, and resolveCombo already overwrites Combo with the
// combo the client addressed precisely so this question has one source.
func (r Resolution) ClientModel() string {
	if r.IsCombo() {
		return r.Combo.Name()
	}
	return ""
}

// answerModel is the model name one answer carries: the combo the client
// addressed when a combo answered, otherwise the name the calling surface already
// used.
//
// The fallback stays with the caller rather than being fixed here because the
// surfaces genuinely disagree today — a translated answer reports the resolved
// model id and a re-framed stream reports the upstream id — and this function
// exists to override both for a combo, not to redefine what a plain model reports.
func answerModel(r Resolution, fallback string) string {
	if name := r.ClientModel(); name != "" {
		return name
	}
	return fallback
}

// stampAnswerModel writes model onto a raw answer the gateway forwards untouched,
// leaving every other member exactly as the upstream wrote it.
//
// Re-encoding the decoded object rather than a typed response is what keeps
// unmodelled fields alive: the passthrough exists to hand the client what the
// upstream said, so narrowing it to the schema's response shape would drop any
// field the schema does not model. The stream re-framer rewrites its identity the
// same way, for the same reason.
//
// An empty name, an unchanged name, or a body that does not decode leaves the
// bytes alone: a call that already reached the upstream and came back with 200 is
// not failed over a label the gateway could not read.
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
