// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability.go
// @for       The model capability the registry cannot carry as data yet: whether
// //
//
//	a model reads images.
//
// @uses      strings (the glob matcher).
// @reason    SPEC-API-001 §7.8 refuses a vision adapter whose models cannot read
//
//	images, and nothing in the registry or the reference's provider files
//	declares that: the knowledge lives in the reference's
//	open-sse/providers/capabilities.js, which resolves it from a table of
//	model-id patterns. This file is that table, ported; capability_resolve.go
//	is the one entry point that reads it.
//	It is a port rather than a regeneration of registry.yaml
//	because the YAML is the provider catalog (what exists, and where it
//	points), while this is a judgement about models the catalog does not
//	enumerate; folding it into the generator would change a committed
//	artifact to answer a question about ids the artifact never lists.
//
//	The port keeps the reference's decision, not its shape: its table
//	lists twelve Claude patterns that all answer "vision", and one rule
//	replaces them. Every fold is justified by the shadowing rules below,
//	and TestCapabilities_MatchesTheReferenceCorpus pins the answers against a
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
	// glm-4.6v/glm-4.5v read images while the `*glm*` rule answers false; the
	// OpenCode Muse Spark pair and union-alpha are declared multimodal by the
	// reference's own MODEL_CAPABILITIES; deepseek's vision variants and the
	// "vision-model" alias are the remaining ids no pattern reaches.
	"glm-4.6v": true, "glm-4.5v": true,
	"glm-5.3-flash":                   true,
	"muse-spark-1.2-contributor-free": true,
	"muse-spark-1.3-contributor-free": true,
	"union-alpha":                     true,
	"deepseek-v4-flash-vision-exp":    true,
	"deepseek-v4.1-flash":             true,
	"deepseek-flash":                  true,
	"vision-model":                    true,
	"kimi-k3":                         true,
	"k3":                              true,
	"kimi-for-coding":                 true,
	"kimi-for-coding-highspeed":       true,
	"kimi-k2.7-code":                  true,
	"kimi-k2.7-code-highspeed":        true,
	// The Claude ids the reference lists explicitly all answer true through
	// `*claude*`, so they are omitted here rather than repeated.
}

// visionRule is one ordered pattern and the answer it fixes.
type visionRule struct {
	pattern string
	vision  bool
}

// VisionCapable reports whether a model reads images.
//
// It is deliberately conservative: an id no rule matches answers false, because
// the caller uses this to refuse a configuration. Guessing "capable" would let
// an operator wire a text-only model into the vision adapter and discover it
// when an image request fails upstream.
//
// It delegates to Capabilities rather than walking the table itself: the
// catalog's `?capability=vision` filter and this predicate are the same
// question, and a second walk here is how the two answers would drift. The
// signature stays because it is the one the vision adapter is wired to.
func VisionCapable(modelID string) bool {
	return Capabilities("", modelID).Vision
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
