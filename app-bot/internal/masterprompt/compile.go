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

func promptTokens(doc Document) []promptToken {
	tokens := []promptToken{
		token(doc.Context.Subject, kindNormal),
		token(doc.Context.Action, kindNormal),
	}
	tokens = append(tokens, splitTokens(doc.Context.Environment)...)
	tokens = append(tokens, compositionTokens(doc.Context.Composition)...)
	tokens = append(tokens, splitTokens(doc.Context.Lighting)...)
	tokens = append(tokens, styleTokens(doc.Context.Style)...)

	if doc.Modality == "video" {
		tokens = append(tokens, videoTokens(doc)...)
	} else {
		for _, detail := range doc.Context.Details {
			tokens = append(tokens, token(detail, kindNormal))
		}
		tokens = append(tokens, splitTokens(qualitySuffix)...)
	}
	tokens = append(tokens, token(avoidPhrase(doc.StopConditions.Negative), kindNormal))
	return tokens
}

// videoTokens renders motion, camera, duration, and leftover details in the
// order the video template states. A camera move arrives as a detail.
func videoTokens(doc Document) []promptToken {
	camera := defaultCamera
	motion := ""
	rest := make([]promptToken, 0, len(doc.Context.Details))
	for _, detail := range doc.Context.Details {
		if isCameraMove(detail) {
			if camera == defaultCamera {
				camera = cameraName(detail)
			}
			continue
		}
		if motion == "" {
			motion = detail
			continue
		}
		rest = append(rest, token(detail, kindNormal))
	}

	tokens := make([]promptToken, 0, len(rest)+3)
	if motion != "" {
		tokens = append(tokens, token(motion, kindNormal))
	}
	tokens = append(tokens, token("camera: "+camera, kindNormal))
	if doc.Output.DurationS != nil {
		tokens = append(tokens, token(fmt.Sprintf("%d seconds", *doc.Output.DurationS), kindNormal))
	}
	return append(tokens, rest...)
}

func compositionTokens(composition Composition) []promptToken {
	tokens := make([]promptToken, 0, 3)
	if composition.Shot != "" {
		tokens = append(tokens, token(phraseValue(composition.Shot)+" shot", kindNormal))
	}
	if composition.Angle != "" {
		tokens = append(tokens, token(phraseValue(composition.Angle)+" angle", kindNormal))
	}
	if composition.AspectRatio != "" {
		tokens = append(tokens, token(composition.AspectRatio+" aspect ratio", kindNormal))
	}
	return tokens
}

func styleTokens(style Style) []promptToken {
	tokens := make([]promptToken, 0, len(style.Palette)+2)
	if style.Medium != "" {
		tokens = append(tokens, token(style.Medium, kindNormal))
	}
	for _, swatch := range style.Palette {
		tokens = append(tokens, token(swatch, kindPalette))
	}
	if style.Mood != "" {
		tokens = append(tokens, token(style.Mood+" mood", kindNormal))
	}
	return tokens
}

func token(value string, kind tokenKind) promptToken {
	return promptToken{text: NormalizeText(value), kind: kind}
}

// splitTokens turns a comma separated field into one dedup unit per piece.
func splitTokens(value string) []promptToken {
	if value == "" {
		return nil
	}
	tokens := make([]promptToken, 0)
	for _, piece := range strings.Split(value, ",") {
		piece = NormalizeText(piece)
		if piece == "" {
			continue
		}
		tokens = append(tokens, promptToken{text: piece, kind: kindNormal})
	}
	return tokens
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

// normalizeMove lowers a detail and keys it against the underscored camera
// vocabulary, so "push in", "PUSH_IN", and "push_in" all read as one move.
func normalizeMove(detail string) string {
	return strings.ReplaceAll(strings.ToLower(NormalizeText(detail)), " ", "_")
}

func isCameraMove(detail string) bool {
	return cameraMoves[normalizeMove(detail)]
}

// cameraName renders the chosen move in prose for the "camera: " line.
func cameraName(detail string) string {
	return phraseValue(normalizeMove(detail))
}
