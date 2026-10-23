// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/fallback.go
// @for       The deterministic builder used when the model is unavailable.
// @uses      fmt, regexp, sort, strings.
// @reason    A builder outage must degrade to a usable draft, so this path extracts what the vocabulary can prove instead of inventing a plan.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Defaults are the per-request choices the fallback keeps when the text says
// nothing about them.
type Defaults struct {
	AspectRatio string
	Style       string
}

var (
	aspectToken   = regexp.MustCompile(`\b(1:1|3:4|4:3|9:16|16:9)\b`)
	durationToken = regexp.MustCompile(`\b(\d{1,2})\s*(detik|seconds|second|sec|s)\b`)
	sentenceBreak = regexp.MustCompile(`[.!?\n]+`)
)

// Fallback builds a document from the user text without a model call.
func Fallback(text, modality string, defaults Defaults) (Document, error) {
	clean := NormalizeText(text)
	if clean == "" {
		return Document{}, &Error{Code: CodeEmpty, Message: "the prompt carries no usable text"}
	}
	clean = truncate(clean, promptCap)

	aspect := extractAspect(clean, defaults.AspectRatio)
	medium := extractMedium(clean, defaults.Style)
	duration := extractDuration(clean, modality)

	task, subject, action := splitSentences(clean)
	family := "visual_designer"
	if modality == "video" {
		family = "motion_designer"
	}

	doc := Document{
		Version:    Version,
		Modality:   modality,
		RoleFamily: family,
		Role:       familyTitle[family],
		Task:       truncate(task, 400),
		Context: Context{
			Subject:     truncate(subject, 300),
			Action:      truncate(action, 200),
			Composition: Composition{Shot: "medium", Angle: "eye_level", AspectRatio: aspect},
			Style:       Style{Medium: medium},
			Details:     extractCamera(clean, modality),
		},
		Reasoning: fmt.Sprintf(
			"Built without a model from the user text, keeping the %s style at %s. The subject stays %q so the frame reads the same after a manual edit.",
			medium, aspect, truncate(subject, 120),
		),
		StopConditions: StopConditions{
			MaxCharacters: budgetFor(modality),
			AspectRatio:   aspect,
			DurationS:     duration,
		},
		Output: Output{
			Asset:       modality,
			AspectRatio: aspect,
			Count:       1,
			Format:      defaultFormat(modality),
			DurationS:   duration,
		},
	}
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func extractAspect(text, fallback string) string {
	if match := aspectToken.FindString(text); match != "" {
		return match
	}
	if aspects[fallback] {
		return fallback
	}
	return "16:9"
}

// extractMedium scans the closed alias list longest first, so "cinematic photo"
// wins over "photo" when both appear.
func extractMedium(text, fallback string) string {
	lower := strings.ToLower(text)
	keys := make([]string, 0, len(mediumAlias))
	for key := range mediumAlias {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, key := range keys {
		if containsWord(lower, key) {
			return mediumAlias[key]
		}
	}
	if mediums[fallback] {
		return fallback
	}
	return "cinematic photo"
}

func extractDuration(text, modality string) *int {
	if modality != "video" {
		return nil
	}
	match := durationToken.FindStringSubmatch(strings.ToLower(text))
	if match == nil {
		seconds := 4
		return &seconds
	}
	seconds := 0
	for _, char := range match[1] {
		seconds = seconds*10 + int(char-'0')
	}
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 10 {
		seconds = 10
	}
	return &seconds
}

// extractCamera keeps the closed camera move so the compiler can lift it out of
// the detail list later.
func extractCamera(text, modality string) []string {
	if modality != "video" {
		return nil
	}
	lower := strings.ToLower(text)
	for move := range cameraMoves {
		if containsWord(lower, phraseValue(move)) {
			return []string{move}
		}
	}
	return []string{defaultCamera}
}

func splitSentences(text string) (task, subject, action string) {
	parts := sentenceBreak.Split(text, -1)
	kept := make([]string, 0, 3)
	for _, part := range parts {
		if clean := NormalizeText(part); clean != "" {
			kept = append(kept, clean)
		}
	}
	if len(kept) == 0 {
		return text, text, ""
	}
	task = kept[0]
	subject = kept[0]
	if len(kept) > 1 {
		action = kept[1]
	}
	if runeLen(task) < 5 {
		task = text
	}
	return task, subject, action
}

func defaultFormat(modality string) string {
	if modality == "video" {
		return "mp4"
	}
	return "png"
}

func containsWord(haystack, needle string) bool {
	index := strings.Index(haystack, needle)
	for index >= 0 {
		before := index == 0 || !isWordChar(haystack[index-1])
		after := index+len(needle) == len(haystack) || !isWordChar(haystack[index+len(needle)])
		if before && after {
			return true
		}
		next := strings.Index(haystack[index+1:], needle)
		if next < 0 {
			return false
		}
		index += 1 + next
	}
	return false
}

func isWordChar(char byte) bool {
	return char == '_' || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
}
