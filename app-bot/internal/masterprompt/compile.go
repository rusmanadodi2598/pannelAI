// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/compile.go
// @for       Deterministic compilation to one prompt string and request parameters.
// @uses      fmt, strings.
// @reason    Same document must produce the same bytes, so section order, joiners, and token dedup are fixed here and pinned by a golden test.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"fmt"
	"strings"
)

// Compiled is the upstream prompt string plus its request parameters.
type Compiled struct {
	Text   string
	Params Params
}

// Params are the values the media adapter sends beside the prompt.
type Params struct {
	AspectRatio string
	Size        string
	Format      string
	Count       int
	DurationS   *int
	Seed        *int64
}

const (
	qualitySuffix = "high detail, sharp focus, professional composition"
	defaultCamera = "static"
)

// tokenKind drives the joiner between neighbouring tokens: two palette
// entries join with " and ", everything else with ", ".
type tokenKind int

const (
	kindNormal tokenKind = iota
	kindPalette
)

// promptToken is one dedup unit in the compiled prompt.
type promptToken struct {
	text string
	kind tokenKind
}

// Compile maps a validated document onto the upstream request.
func Compile(doc Document) (Compiled, error) {
	text := compileText(doc)

	limit := budgetFor(doc.Modality)
	if doc.StopConditions.MaxCharacters < limit {
		limit = doc.StopConditions.MaxCharacters
	}
	if runeLen(text) > limit {
		return Compiled{}, &Error{
			Code:    CodeBudget,
			Message: fmt.Sprintf("compiled prompt is %d characters, budget is %d", runeLen(text), limit),
		}
	}

	return Compiled{Text: text, Params: paramsFor(doc)}, nil
}

func compileText(doc Document) string {
	return joinTokens(dedupeTokens(promptTokens(doc)))
}

// dedupeTokens drops a repeated token case-insensitively, keeping the first.
func dedupeTokens(tokens []promptToken) []promptToken {
	seen := map[string]bool{}
	kept := make([]promptToken, 0, len(tokens))
	for _, tk := range tokens {
		key := strings.ToLower(tk.text)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, tk)
	}
	return kept
}

// joinTokens writes the joiner before each token, so two palette entries
// read "a and b" and anything else reads "a, b".
func joinTokens(tokens []promptToken) string {
	if len(tokens) == 0 {
		return ""
	}
	joined := make([]string, 0, len(tokens))
	joined = append(joined, tokens[0].text)
	for i := 1; i < len(tokens); i++ {
		sep := ", "
		if tokens[i].kind == kindPalette && tokens[i-1].kind == kindPalette {
			sep = " and "
		}
		joined = append(joined, sep+tokens[i].text)
	}
	return strings.Join(joined, "")
}

// avoidPhrase merges the document negatives with the baseline, keeping the
// first occurrence of a token so a repeat never reaches the provider twice.
func avoidPhrase(negative []string) string {
	merged := append(append([]string{}, negative...), baselineNegative...)
	unique := dedupe(merged)
	if len(unique) == 0 {
		return ""
	}
	return "avoid: " + strings.Join(unique, ", ")
}

func dedupe(values []string) []string {
	kept := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		key := strings.ToLower(NormalizeText(value))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, key)
	}
	return kept
}

func paramsFor(doc Document) Params {
	sizes := imageSizes
	if doc.Modality == "video" {
		sizes = videoSizes
	}
	return Params{
		AspectRatio: doc.Output.AspectRatio,
		Size:        sizes[doc.Output.AspectRatio],
		Format:      doc.Output.Format,
		Count:       doc.Output.Count,
		DurationS:   doc.Output.DurationS,
		Seed:        doc.Seed,
	}
}

// phraseValue turns an enum member into prose: eye_level reads as eye level.
func phraseValue(value string) string {
	return strings.ReplaceAll(value, "_", " ")
}
