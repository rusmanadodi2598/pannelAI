// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/compile_segments.go
// @for       The per-section token builders the compiler assembles in a fixed order.
// @uses      fmt, strings.
// @reason    Keeping the section builders beside the join and dedup logic lets each sit under the line limit without changing the prompt bytes.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"fmt"
	"strings"
)

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
