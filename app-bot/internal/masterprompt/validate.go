// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/validate.go
// @for       Every field and cross-field rule from SPEC-BOT-001 §6.2.
// @uses      fmt, strings.
// @reason    A builder reply is untrusted data, so each limit is enforced here once and the compiler can assume a clean document.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"fmt"
	"strings"
)

// Validate applies the schema rules to a normalized document.
func Validate(doc Document) error {
	if doc.Version != Version {
		return fieldError(CodeInvalid, "version", "must be "+Version)
	}
	if hasURL(doc) {
		return fieldError(CodeUnsupported, "url", "a link is not accepted in any field")
	}

	modality := doc.Modality
	if modality != "image" && modality != "video" {
		return fieldError(CodeInvalid, "modality", "must be image or video")
	}
	if _, ok := familyDescriptor[doc.RoleFamily]; !ok {
		return fieldError(CodeInvalid, "role_family", "is not in the closed family list")
	}
	if doc.Task == "" && doc.Context.Subject == "" && doc.Role == "" && doc.Reasoning == "" {
		return &Error{Code: CodeEmpty, Message: "the prompt carries no usable text"}
	}
	if err := validateRole(doc); err != nil {
		return err
	}
	if err := validateTask(doc); err != nil {
		return err
	}
	if err := validateContext(doc); err != nil {
		return err
	}
	if err := validateReasoning(doc); err != nil {
		return err
	}
	if err := validateStop(doc); err != nil {
		return err
	}
	if err := validateOutput(doc); err != nil {
		return err
	}
	return validateAgreement(doc)
}

func validateRole(doc Document) error {
	if n := runeLen(doc.Role); n < 3 || n > 200 {
		return fieldError(CodeInvalid, "role", "must be 3 to 200 characters")
	}
	lower := strings.ToLower(doc.Role)
	for _, word := range nonVisualWords {
		if strings.Contains(lower, word) {
			return fieldError(CodeRoleScope, "role", "names a non-visual profession")
		}
	}
	return nil
}

func validateTask(doc Document) error {
	if n := runeLen(doc.Task); n < 5 || n > 400 {
		return fieldError(CodeInvalid, "task", "must be 5 to 400 characters")
	}
	return nil
}

func validateContext(doc Document) error {
	if n := runeLen(doc.Context.Subject); n < 1 || n > 300 {
		return fieldError(CodeInvalid, "context.subject", "must be 1 to 300 characters")
	}
	if n := runeLen(doc.Context.Action); n > 200 {
		return fieldError(CodeInvalid, "context.action", "must be at most 200 characters")
	}
	if n := runeLen(doc.Context.Environment); n > 200 {
		return fieldError(CodeInvalid, "context.environment", "must be at most 200 characters")
	}
	if n := runeLen(doc.Context.Lighting); n > 120 {
		return fieldError(CodeInvalid, "context.lighting", "must be at most 120 characters")
	}
	composition := doc.Context.Composition
	if !shots[composition.Shot] {
		return fieldError(CodeInvalid, "context.composition.shot", "is not in the closed list")
	}
	if !angles[composition.Angle] {
		return fieldError(CodeInvalid, "context.composition.angle", "is not in the closed list")
	}
	if !aspects[composition.AspectRatio] {
		return fieldError(CodeInvalid, "context.composition.aspect_ratio", "is not in the closed list")
	}
	if !mediums[doc.Context.Style.Medium] {
		return fieldError(CodeInvalid, "context.style.medium", "is not in the closed list")
	}
	if len(doc.Context.Style.Palette) > 4 {
		return fieldError(CodeInvalid, "context.style.palette", "accepts at most 4 entries")
	}
	for _, entry := range doc.Context.Style.Palette {
		if runeLen(entry) > 32 {
			return fieldError(CodeInvalid, "context.style.palette", "each entry must be at most 32 characters")
		}
	}
	if runeLen(doc.Context.Style.Mood) > 48 {
		return fieldError(CodeInvalid, "context.style.mood", "must be at most 48 characters")
	}
	if len(doc.Context.Details) > 6 {
		return fieldError(CodeInvalid, "context.details", "accepts at most 6 entries")
	}
	for _, entry := range doc.Context.Details {
		if runeLen(entry) > 120 {
			return fieldError(CodeInvalid, "context.details", "each entry must be at most 120 characters")
		}
	}
	return nil
}

func validateReasoning(doc Document) error {
	if n := runeLen(doc.Reasoning); n < 20 || n > 600 {
		return fieldError(CodeInvalid, "reasoning", "must be 20 to 600 characters")
	}
	if !mentionsContext(doc) {
		return fieldError(CodeInvalid, "reasoning", "names no element from the context")
	}
	return nil
}

func validateStop(doc Document) error {
	if len(doc.StopConditions.Negative) > 10 {
		return fieldError(CodeInvalid, "stop_conditions.negative", "accepts at most 10 entries")
	}
	for _, entry := range doc.StopConditions.Negative {
		if runeLen(entry) > 60 {
			return fieldError(CodeInvalid, "stop_conditions.negative", "each entry must be at most 60 characters")
		}
	}
	if doc.StopConditions.MaxCharacters < 500 || doc.StopConditions.MaxCharacters > 3500 {
		return fieldError(CodeInvalid, "stop_conditions.max_characters", "must be 500 to 3500")
	}
	if !aspects[doc.StopConditions.AspectRatio] {
		return fieldError(CodeInvalid, "stop_conditions.aspect_ratio", "is not in the closed list")
	}
	return validateDuration("stop_conditions.duration_s", doc.Modality, doc.StopConditions.DurationS)
}

func validateOutput(doc Document) error {
	if doc.Output.Asset != doc.Modality {
		return fieldError(CodeInvalid, "output.asset", "must equal the modality")
	}
	if !aspects[doc.Output.AspectRatio] {
		return fieldError(CodeInvalid, "output.aspect_ratio", "is not in the closed list")
	}
	if doc.Output.Count != 1 {
		return fieldError(CodeUnsupported, "output.count", "only one asset is supported")
	}
	if !allowedFormat(doc.Output.Asset, doc.Output.Format) {
		return fieldError(CodeInvalid, "output.format", "is not allowed for this asset")
	}
	return validateDuration("output.duration_s", doc.Modality, doc.Output.DurationS)
}

func validateAgreement(doc Document) error {
	if doc.StopConditions.AspectRatio != doc.Output.AspectRatio {
		return fieldError(CodeInvalid, "aspect_ratio", "stop conditions and output disagree")
	}
	if doc.Context.Composition.AspectRatio != doc.Output.AspectRatio {
		return fieldError(CodeInvalid, "aspect_ratio", "composition and output disagree")
	}
	if doc.Modality == "video" && (doc.Output.DurationS == nil || doc.StopConditions.DurationS == nil) {
		return fieldError(CodeUnsupported, "duration_s", "video requires a duration")
	}
	return nil
}

func validateDuration(field, modality string, value *int) error {
	if modality == "image" {
		if value != nil {
			return fieldError(CodeUnsupported, field, "image must not carry a duration")
		}
		return nil
	}
	if value == nil {
		return nil
	}
	if *value < 1 || *value > 10 {
		return fieldError(CodeInvalid, field, "must be 1 to 10 seconds")
	}
	return nil
}

func allowedFormat(asset, format string) bool {
	for _, allowed := range outputFormats[asset] {
		if format == allowed {
			return true
		}
	}
	return false
}

// mentionsContext is the cheap relevance check from §6.3.1: reasoning that
// shares no word with the context is filler.
func mentionsContext(doc Document) bool {
	contextWords := map[string]bool{}
	sources := []string{
		doc.Context.Subject, doc.Context.Action, doc.Context.Environment,
		doc.Context.Lighting, doc.Context.Style.Medium, doc.Context.Style.Mood,
		doc.Task,
	}
	sources = append(sources, doc.Context.Details...)
	sources = append(sources, doc.Context.Style.Palette...)
	for _, source := range sources {
		for _, word := range words(source) {
			contextWords[word] = true
		}
	}
	for _, word := range words(doc.Reasoning) {
		if contextWords[word] {
			return true
		}
	}
	return false
}

func fieldError(code, field, rule string) error {
	return &Error{Code: code, Message: fmt.Sprintf("%s %s", field, rule)}
}
