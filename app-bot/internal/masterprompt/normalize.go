// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/normalize.go
// @for       One normalization pipeline for text, lists, and the whole document.
// @uses      regexp, strings, unicode, golang.org/x/text/unicode/norm.
// @reason    Decode, fallback, and compile must agree on what "the same text" means, so control characters, whitespace, and Unicode form are settled in one place.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	spaceRun  = regexp.MustCompile(`\s+`)
	urlShape  = regexp.MustCompile(`(?i)(https?://|ftp://|www\.)\S+`)
	wordShape = regexp.MustCompile(`[a-z0-9]{4,}`)
)

// NormalizeText applies NFC, drops control characters, and collapses
// whitespace, so an empty result means the field carried nothing usable.
func NormalizeText(value string) string {
	cleaned := strings.Map(func(char rune) rune {
		if unicode.IsControl(char) {
			return ' '
		}
		return char
	}, norm.NFC.String(value))
	return spaceRun.ReplaceAllString(strings.TrimSpace(cleaned), " ")
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := NormalizeText(value)
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDocument(doc Document) Document {
	doc.Version = NormalizeText(doc.Version)
	doc.Modality = strings.ToLower(NormalizeText(doc.Modality))
	doc.RoleFamily = strings.ToLower(NormalizeText(doc.RoleFamily))
	doc.Role = NormalizeText(doc.Role)
	doc.Task = NormalizeText(doc.Task)
	doc.Reasoning = NormalizeText(doc.Reasoning)

	doc.Context.Subject = NormalizeText(doc.Context.Subject)
	doc.Context.Action = NormalizeText(doc.Context.Action)
	doc.Context.Environment = NormalizeText(doc.Context.Environment)
	doc.Context.Lighting = NormalizeText(doc.Context.Lighting)
	doc.Context.Composition.Shot = strings.ToLower(NormalizeText(doc.Context.Composition.Shot))
	doc.Context.Composition.Angle = strings.ToLower(NormalizeText(doc.Context.Composition.Angle))
	doc.Context.Composition.AspectRatio = NormalizeText(doc.Context.Composition.AspectRatio)
	doc.Context.Style.Medium = strings.ToLower(NormalizeText(doc.Context.Style.Medium))
	doc.Context.Style.Mood = NormalizeText(doc.Context.Style.Mood)
	doc.Context.Style.Palette = normalizeList(doc.Context.Style.Palette)
	doc.Context.Details = normalizeList(doc.Context.Details)

	doc.StopConditions.Negative = normalizeList(doc.StopConditions.Negative)
	doc.StopConditions.AspectRatio = NormalizeText(doc.StopConditions.AspectRatio)
	doc.Output.Asset = strings.ToLower(NormalizeText(doc.Output.Asset))
	doc.Output.AspectRatio = NormalizeText(doc.Output.AspectRatio)
	doc.Output.Format = strings.ToLower(NormalizeText(doc.Output.Format))
	return doc
}

// remapRole keeps a product-named title from reaching the preview while still
// saying who is creating the asset.
func remapRole(doc Document) Document {
	lower := strings.ToLower(doc.Role)
	for _, word := range productWords {
		if strings.Contains(lower, word) {
			doc.Role = familyTitle[doc.RoleFamily]
			return doc
		}
	}
	if strings.Contains(lower, "only ") {
		doc.Role = familyTitle[doc.RoleFamily]
	}
	return doc
}

// hasURL reports whether any string field carries a link. The builder is not a
// fetch tool, so a link is refused rather than compiled.
func hasURL(doc Document) bool {
	fields := []string{
		doc.Role, doc.Task, doc.Reasoning, doc.Context.Subject, doc.Context.Action,
		doc.Context.Environment, doc.Context.Lighting, doc.Context.Style.Mood,
	}
	fields = append(fields, doc.Context.Style.Palette...)
	fields = append(fields, doc.Context.Details...)
	fields = append(fields, doc.StopConditions.Negative...)
	for _, field := range fields {
		if urlShape.MatchString(field) {
			return true
		}
	}
	return false
}

// words splits a field into lowercase tokens used for the reasoning relevance
// check and for token deduplication.
func words(value string) []string {
	return wordShape.FindAllString(strings.ToLower(value), -1)
}

func runeLen(value string) int { return len([]rune(value)) }

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
