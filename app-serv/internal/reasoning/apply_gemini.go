// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/apply_gemini.go
// @for       The Gemini writers: the thinkingConfig block, the envelope it may
//
//	sit in, and the output-token floor it needs.
//
// @uses      encoding/json.
// @reason    SPEC-API-001 §7.15 gives Gemini two thinking formats (a level and a
//
//	budget), and both write into a nested generationConfig that the
//	payload may wrap in a request envelope. Those three facts are one
//	subject, so they live together, and apply_helpers.go stays inside
//	the AGENTS.md §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "encoding/json"

// setGeminiThinking writes Gemini's thinkingConfig, under the request envelope
// when the payload wraps one and under generationConfig otherwise.
func setGeminiThinking(body map[string]json.RawMessage, thinking map[string]any) {
	target, nested := geminiConfigTarget(body)
	config := ensureObject(target, "generationConfig")
	config["thinkingConfig"] = mustJSON(thinking)
	target["generationConfig"] = mustJSON(config)
	if nested {
		body["request"] = mustJSON(target)
	}
}

// ensureGeminiOutputFloor raises maxOutputTokens to the floor a thinking budget
// needs, which the reference clamps to the model's declared output ceiling; this
// port does not carry that ceiling, so the floor is used as it stands. The
// divergence is unreachable with the curated registry: a Gemini-format model is
// served on the OpenAI wire, where the format resolves to openai instead.
func ensureGeminiOutputFloor(body map[string]json.RawMessage, floor int) {
	target, nested := geminiConfigTarget(body)
	config := ensureObject(target, "generationConfig")
	if current, found := intField(config, "maxOutputTokens"); !found || current < floor {
		config["maxOutputTokens"] = mustJSON(floor)
	}
	target["generationConfig"] = mustJSON(config)
	if nested {
		body["request"] = mustJSON(target)
	}
}

// geminiConfigTarget answers the object that holds generationConfig: the
// request envelope when the payload wraps one, the body otherwise. The second
// result reports which, so the caller knows whether to write the envelope back.
func geminiConfigTarget(body map[string]json.RawMessage) (map[string]json.RawMessage, bool) {
	if request, ok := objectField(body, "request"); ok {
		return request, true
	}
	return body, false
}

// geminiLevelOutputFloor is the output-token floor for a Gemini thinking level.
func geminiLevelOutputFloor(level string) int {
	switch level {
	case "minimal":
		return 4096
	case "low":
		return 8192
	case "medium":
		return 16384
	default:
		return 65535
	}
}

// geminiBudgetOutputFloor is the output-token floor for a Gemini thinking
// budget. -1 is the upstream's own default, which gets the middle floor.
func geminiBudgetOutputFloor(budget int) int {
	switch {
	case budget < 0:
		return 32768
	case budget <= 1024:
		return 8192
	case budget <= 8192:
		return 16384
	case budget <= 24576:
		return 32768
	default:
		return 65535
	}
}
