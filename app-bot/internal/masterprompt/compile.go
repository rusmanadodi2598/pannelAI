// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/compile.go
// @for       Deterministic compilation to one prompt string and request parameters.
// @uses      fmt, strings.
// @reason    Same document must produce the same bytes, so section order and separators are fixed here and pinned by a golden test.
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

// Compile maps a validated document onto the upstream request.
func Compile(doc Document) (Compiled, error) {
	segments := promptSegments(doc)
	text := joinSegments(segments)

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

func promptSegments(doc Document) []string {
	segments := []string{
		doc.Context.Subject,
		doc.Context.Action,
		doc.Context.Environment,
		compositionPhrase(doc.Context.Composition),
		doc.Context.Lighting,
		stylePhrase(doc.Context.Style),
	}
	if doc.Modality == "video" {
		segments = append(segments, videoSegments(doc)...)
	} else {
		segments = append(segments, doc.Context.Details...)
		segments = append(segments, qualitySuffix)
	}
	return append(segments, avoidPhrase(doc.StopConditions.Negative))
}

// videoSegments renders motion, camera, duration, and the leftover details in
// the order the video template states.
func videoSegments(doc Document) []string {
	camera := defaultCamera
	motion := ""
	rest := make([]string, 0, len(doc.Context.Details))
	for _, detail := range doc.Context.Details {
		key := phraseValue(strings.ToLower(detail))
		switch {
		case cameraMoves[key] && camera == defaultCamera:
			camera = key
		case motion == "" && cameraMoves[key] == false:
			motion = detail
		default:
			rest = append(rest, detail)
		}
	}

	segments := make([]string, 0, len(rest)+3)
	if motion != "" {
		segments = append(segments, motion)
	}
	segments = append(segments, "camera: "+camera)
	if doc.Output.DurationS != nil {
		segments = append(segments, fmt.Sprintf("%d seconds", *doc.Output.DurationS))
	}
	return append(segments, rest...)
}

func compositionPhrase(composition Composition) string {
	parts := make([]string, 0, 3)
	if composition.Shot != "" {
		parts = append(parts, phraseValue(composition.Shot)+" shot")
	}
	if composition.Angle != "" {
		parts = append(parts, phraseValue(composition.Angle)+" angle")
	}
	if composition.AspectRatio != "" {
		parts = append(parts, composition.AspectRatio+" aspect ratio")
	}
	return strings.Join(parts, ", ")
}

func stylePhrase(style Style) string {
	parts := make([]string, 0, 3)
	if style.Medium != "" {
		parts = append(parts, style.Medium)
	}
	if len(style.Palette) > 0 {
		parts = append(parts, strings.Join(style.Palette, " and "))
	}
	if style.Mood != "" {
		parts = append(parts, style.Mood+" mood")
	}
	return strings.Join(parts, ", ")
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

// joinSegments skips empty parts and drops a repeated token, so the prompt
// never carries ", ," or the same phrase twice under a different case.
func joinSegments(segments []string) string {
	kept := make([]string, 0, len(segments))
	seen := map[string]bool{}
	for _, segment := range segments {
		clean := NormalizeText(segment)
		if clean == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSuffix(clean, ","))
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, clean)
	}
	return strings.Join(kept, ", ")
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
