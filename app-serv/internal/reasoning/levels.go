// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/levels.go
// @for       The level and budget vocabularies the thinking formats share.
// @uses      (no imports).
// @reason    SPEC-API-001 §7.14 stores one thinking mode per provider and §7.15
//
//	carries the reference's suffix; both end as a field an upstream reads,
//	and the level a caller names is not always the number that upstream
//	wants. The maps are the reference's own (thinking.js), ported whole so
//	a level the panel offers cannot mean a different budget here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "strings"

// EffortLevels is the discrete effort vocabulary in escalation order, which is
// the reference's EFFORT_LEVELS.
var EffortLevels = []string{"minimal", "low", "medium", "high", "xhigh", "max"}

// LevelToBudget maps a level to the budget_tokens the web-standard formats
// (Anthropic, Gemini) document. Zero means "no thinking"; an absent key means
// the level is not recognized, which is a different answer from a zero budget.
var LevelToBudget = map[string]int{
	"none":    0,
	"minimal": 512,
	"low":     1024,
	"medium":  8192,
	"high":    24576,
	"xhigh":   32768,
	"max":     128000,
}

// EffortToBudget answers a level's budget. The second result is false when the
// level is unknown, which the caller must not read as "no thinking".
func EffortToBudget(effort string) (int, bool) {
	budget, found := LevelToBudget[strings.ToLower(strings.TrimSpace(effort))]
	return budget, found
}

// EffortToThinkingLevel maps an effort onto Gemini's thinkingLevel enum
// (minimal|low|medium|high). Gemini 3 cannot fully disable thinking, so
// "none"/"off" become minimal and the two top levels clamp to high.
func EffortToThinkingLevel(effort string) string {
	level := strings.ToLower(strings.TrimSpace(effort))
	switch level {
	case "none", "off":
		return "minimal"
	case "xhigh", "max":
		return "high"
	default:
		return level
	}
}

// BudgetToLevel answers the nearest discrete level for a numeric budget, by the
// reference's thresholds: the midpoints between the LevelToBudget values, so
// max stays reachable. A non-positive budget answers "", meaning no reasoning.
func BudgetToLevel(budget int) string {
	switch {
	case budget <= 0:
		return ""
	case budget <= 768:
		return "minimal"
	case budget <= 4096:
		return "low"
	case budget <= 16384:
		return "medium"
	case budget <= 28672:
		return "high"
	case budget <= 80384:
		return "xhigh"
	default:
		return "max"
	}
}

// BudgetToEffort answers the coarse three-level map the antigravity reverse
// translation uses. A non-positive budget answers "".
func BudgetToEffort(budget int) string {
	switch {
	case budget <= 0:
		return ""
	case budget <= 2048:
		return "low"
	case budget <= 16384:
		return "medium"
	default:
		return "high"
	}
}

// NormalizeOpenAILevel clamps the two levels OpenAI does not enumerate. A level
// the model's own list accepts is passed through, "ultra" becomes "max" where
// max is supported, and everything else becomes xhigh.
func NormalizeOpenAILevel(level string, supported []string) string {
	if level != "max" && level != "ultra" {
		return level
	}
	for _, known := range supported {
		if known == level {
			return level
		}
	}
	if level == "ultra" {
		for _, known := range supported {
			if known == "max" {
				return "max"
			}
		}
	}
	return "xhigh"
}
