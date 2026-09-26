// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/intent.go
// @for       Reading the reasoning intent a client's own body carries.
// @uses      encoding/json, strings.
// @reason    SPEC-API-001 §7.15 ports the reference's rule that a client's own
//
//	reasoning intent is left alone: the provider mode is injected only when
//	the client asked for nothing (chatCore.js:107-117), and the per-model
//	suffix outranks both (thinkingUnified.js applyThinking). One extractor
//	for every client wire is what makes that rule one comparison rather
//	than a branch per format.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import (
	"encoding/json"
	"strings"
)

// Config is a resolved reasoning intent, in the reference's unified shape:
// Mode is one of "none" (thinking off), "auto" (the upstream's own default),
// "level" (a named level in Level), or "budget" (a token count in Budget).
type Config struct {
	Mode   string
	Level  string
	Budget int
}

// Extract reads the intent a client body carries, in the reference's own
// priority order: an explicit Claude effort, an OpenAI effort, a Claude
// thinking object, a Gemini thinking config, then the Qwen flags. It answers
// nil when the client asked for nothing, which is the state the provider mode
// is injected into.
func Extract(raw []byte) *Config {
	body, ok := decodeObject(raw)
	if !ok {
		return nil
	}
	return extractFromObject(body)
}

// extractFromObject is Extract over an already decoded body, so a caller that
// decoded for another reason does not decode twice.
func extractFromObject(body map[string]json.RawMessage) *Config {
	// Claude's output_config.effort is explicit and outranks the adaptive
	// thinking switch that usually accompanies it.
	if output, ok := objectField(body, "output_config"); ok {
		if effort := stringField(output, "effort"); effort != "" {
			return configFromEffort(effort)
		}
	}
	// OpenAI chat and Responses shapes. z.ai sends both a thinking object and
	// reasoning.effort, so the effort is read first.
	if effort := stringField(body, "reasoning_effort"); effort != "" {
		return configFromEffort(effort)
	}
	if reasoning, ok := objectField(body, "reasoning"); ok {
		if effort := stringField(reasoning, "effort"); effort != "" {
			return configFromEffort(effort)
		}
	}
	// Anthropic's extended-thinking control. A budget makes it a budget; a
	// bare enabled/adaptive switch is the upstream's own default.
	if thinking, ok := objectField(body, "thinking"); ok {
		switch stringField(thinking, "type") {
		case "disabled":
			return &Config{Mode: "none"}
		case "adaptive", "enabled":
			if budget, found := intField(thinking, "budget_tokens"); found && budget > 0 {
				return &Config{Mode: "budget", Budget: budget}
			}
			return &Config{Mode: "auto"}
		}
	}
	// Gemini's thinkingConfig, at the top level, under generationConfig, or
	// under a request envelope.
	if config, ok := geminiThinkingConfig(body); ok {
		if level := stringField(config, "thinkingLevel"); level != "" {
			return &Config{Mode: "level", Level: strings.ToLower(level)}
		}
		if budget, found := intField(config, "thinkingBudget"); found {
			switch {
			case budget == 0:
				return &Config{Mode: "none"}
			case budget < 0:
				return &Config{Mode: "auto"}
			default:
				return &Config{Mode: "budget", Budget: budget}
			}
		}
	}
	// Qwen's flag pair.
	if raw, ok := body["enable_thinking"]; ok {
		var enabled bool
		if json.Unmarshal(raw, &enabled) == nil {
			if !enabled {
				return &Config{Mode: "none"}
			}
			if budget, found := intField(body, "thinking_budget"); found && budget > 0 {
				return &Config{Mode: "budget", Budget: budget}
			}
			return &Config{Mode: "auto"}
		}
	}
	return nil
}

// configFromEffort reads an effort word the way the reference does: the two
// off words are "none", "auto" is the upstream default, and anything else is a
// level, left as written because the format decides whether it is valid.
func configFromEffort(effort string) *Config {
	switch word := strings.ToLower(strings.TrimSpace(effort)); word {
	case "none", "off":
		return &Config{Mode: "none"}
	case "auto":
		return &Config{Mode: "auto"}
	default:
		return &Config{Mode: "level", Level: word}
	}
}

// geminiThinkingConfig finds Gemini's thinking block in either of the two
// places the reference looks, including the request envelope the CLI and
// antigravity payloads wrap their request in.
func geminiThinkingConfig(body map[string]json.RawMessage) (map[string]json.RawMessage, bool) {
	if config, ok := objectField(body, "thinkingConfig"); ok {
		return config, true
	}
	if generation, ok := objectField(body, "generationConfig"); ok {
		if config, ok := objectField(generation, "thinkingConfig"); ok {
			return config, true
		}
	}
	if request, ok := objectField(body, "request"); ok {
		if generation, ok := objectField(request, "generationConfig"); ok {
			if config, ok := objectField(generation, "thinkingConfig"); ok {
				return config, true
			}
		}
	}
	return nil, false
}
