// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability.go
// @for       The model capability the registry cannot carry as data yet: whether
// //
//
//	a model reads images.
//
// @uses      strings.
// @reason    SPEC-API-001 §7.8 refuses a vision adapter whose models cannot read
//
//	images, and nothing in the registry or the reference's provider files
//	declares that: the knowledge lives in the reference's
//	open-sse/providers/capabilities.js, which resolves it from a table of
//	model-id patterns. This file ports the vision half of that table, so
//	the rule is stated once and testable instead of guessed at the call
//	site. It is a port rather than a regeneration of registry.yaml
//	because the YAML is the provider catalog (what exists, and where it
//	points), while this is a judgement about models the catalog does not
//	enumerate; folding it into the generator would change a committed
//	artifact to answer a question about ids the artifact never lists.
//
//	The port keeps the reference's decision, not its shape: its table
//	lists twelve Claude patterns that all answer "vision", and one rule
//	replaces them. Every fold is justified by the shadowing rules below,
//	and TestVisionCapable_MatchesTheReference pins the answers against a
//	corpus generated from capabilities.js itself.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-18
package registry

import "strings"

// visionExactIDs are the ids whose answer the pattern table alone gets wrong.
// The reference also lists the Claude 4.6/4.7 ids and "coder-model" here, and
// both are omitted deliberately: every Claude id answers true through the
// `*claude*` rule below, and "coder-model" answers false, which is the default.
var visionExactIDs = map[string]bool{
	// The one GLM variant that reads images; the `*glm*` rule says false.
	"glm-4.6v": true,
	// The registry's Qwen vision alias, which no pattern matches.
	"vision-model": true,
}

// visionRule is one ordered pattern and the answer it fixes.
type visionRule struct {
	pattern string
	vision  bool
}

// visionRules is the reference's PATTERN_CAPABILITIES reduced to its vision
// decision, in its order: the first match wins, and a match returns even when
// the reference's entry carried no vision flag — that flag merges to the false
// floor, which is why "*gpt-5*image*" answers false rather than falling through
// to "*gpt-5*".
var visionRules = []visionRule{
	// OpenAI. The image and codex variants must stay ahead of "*gpt-5*".
	{pattern: "*gpt-5*image*", vision: false},
	{pattern: "*gpt-5*codex*", vision: false},
	{pattern: "*gpt-5*", vision: true},
	{pattern: "*gpt-4o*", vision: true},
	{pattern: "*gpt-4.1*", vision: true},
	{pattern: "*gpt-4-turbo*", vision: true},
	{pattern: "*gpt-4*", vision: false},
	{pattern: "*gpt-3.5*", vision: false},
	{pattern: "*gpt-oss*", vision: false},
	// The o-series: o1-mini is the one text-only member.
	{pattern: "*o1-mini*", vision: false},
	{pattern: "*o1*", vision: true},
	{pattern: "*o3*", vision: true},
	{pattern: "*o4*", vision: true},
	// Grok, whose code and image variants are text-only.
	{pattern: "*grok*image*", vision: false},
	{pattern: "*grok-code*", vision: false},
	{pattern: "*grok*", vision: true},
	// Qwen: the vision, max, and plus variants read images; the coder and 235b
	// families do not, and QwQ is thinking-only.
	{pattern: "*qwen*vl*", vision: true},
	{pattern: "*qwen*max*", vision: true},
	{pattern: "*qwen*plus*", vision: true},
	{pattern: "*qwen*", vision: false},
	{pattern: "*qwq*", vision: false},
	// Kimi: K2 reads images, the older family does not.
	{pattern: "*kimi*k2*", vision: true},
	{pattern: "*kimi*", vision: false},
	// Families whose every member reads images.
	{pattern: "*claude*", vision: true},
	{pattern: "*gemini*", vision: true},
	{pattern: "*gemma*", vision: true},
	{pattern: "*nanobanana*", vision: true},
	{pattern: "*mimo*", vision: true},
	{pattern: "*llama-4*", vision: true},
	{pattern: "*llama*", vision: false},
	{pattern: "*mistral-large*", vision: true},
	{pattern: "*codestral*", vision: false},
	{pattern: "*mistral*", vision: false},
	{pattern: "*command-a-vision*", vision: true},
	{pattern: "*command*", vision: false},
	// Everything else answers false: the reference's remaining rules (glm,
	// deepseek, minimax, sonar, hunyuan, step, nemotron, ling) carry no vision
	// flag at all, which merges to the same false floor.
}

// VisionCapable reports whether a model reads images.
//
// It is deliberately conservative: an id no rule matches answers false, because
// the caller uses this to refuse a configuration. Guessing "capable" would let
// an operator wire a text-only model into the vision adapter and discover it
// when an image request fails upstream.
func VisionCapable(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if known, found := visionExactIDs[id]; found {
		return known
	}
	for _, rule := range visionRules {
		if matchesGlob(rule.pattern, id) {
			return rule.vision
		}
	}
	return false
}

// matchesGlob reports whether pattern matches value, case-insensitively, with
// `*` standing for any run of characters and the match anchored at both ends —
// the rule the reference applies in pricing.js:213-216. Both arguments are
// expected lower-cased by the caller.
func matchesGlob(pattern, value string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	if !strings.HasPrefix(value, parts[0]) {
		return false
	}
	value = value[len(parts[0]):]

	last := parts[len(parts)-1]
	if !strings.HasSuffix(value, last) {
		return false
	}
	value = value[:len(value)-len(last)]

	// The middle parts must appear in order, and may not overlap: consuming
	// each match keeps "*ab*b*" from matching "ab" twice.
	for _, part := range parts[1 : len(parts)-1] {
		if part == "" {
			continue
		}
		found := strings.Index(value, part)
		if found < 0 {
			return false
		}
		value = value[found+len(part):]
	}
	return true
}
