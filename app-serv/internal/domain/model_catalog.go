// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/model_catalog.go
// @for       The model reference and capability value objects, and the shape
//
//	rules every model string in the catalog must satisfy.
//
// @uses      internal/domain (AppError constructors), sort, strings.
// @reason    SPEC-API-001 §7.6 merges the embedded registry, models_custom, and
//
//	models_disabled into one catalog addressed by (provider_id,
//	model_id), and §7.7 addresses combos and aliases by the same
//	strings; declaring the reference and its parse rule once here is
//	what stops four call sites from splitting a model string
//	differently.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"sort"
	"strings"
)

// CustomModelIDPrefix is the type prefix for a models_custom id. SPEC-API-001
// §4 fixes prefixes for the ids the API mints, and the panel addresses a custom
// model by this id in DELETE /api/v1/models/custom/{id}.
const CustomModelIDPrefix = "mdl_"

// modelSegmentMax bounds one segment of a model string.
const modelSegmentMax = 200

// ModelRef is the (provider, model) pair every model is addressed by. The pair
// is the key everywhere: registry rows, custom rows, disabled rows, and
// coverage rows all use it, so the merge needs one shape.
type ModelRef struct {
	providerID string
	modelID    string
}

// NewModelRef builds a reference from its two parts.
//
// A provider id may not contain "/" because it is the namespace segment of a
// model string; a model id may, because the registry declares ids like
// "openai/text-embedding-3-large" whose namespace is the provider. That
// asymmetry is why ParseModelRef splits on the FIRST slash only.
func NewModelRef(providerID, modelID string) (ModelRef, error) {
	provider := strings.TrimSpace(providerID)
	model := strings.TrimSpace(modelID)
	if provider == "" {
		return ModelRef{}, NewValidationError("provider_id is required")
	}
	if model == "" {
		return ModelRef{}, NewValidationError("model_id is required")
	}
	if strings.Contains(provider, "/") || hasWhitespace(provider) {
		return ModelRef{}, NewValidationError("provider_id must not contain a slash or whitespace")
	}
	if hasWhitespace(model) {
		return ModelRef{}, NewValidationError("model_id must not contain whitespace")
	}
	if len(provider) > modelSegmentMax || len(model) > modelSegmentMax {
		return ModelRef{}, NewValidationError("model reference is too long")
	}
	return ModelRef{providerID: provider, modelID: model}, nil
}

// ParseModelRef reads the "provider/model" form. A string without a slash, or
// with an empty segment on either side, is not a reference: the caller decides
// whether it is a combo name or an alias instead.
func ParseModelRef(raw string) (ModelRef, error) {
	trimmed := strings.TrimSpace(raw)
	idx := strings.Index(trimmed, "/")
	if idx < 0 {
		return ModelRef{}, NewValidationError("model reference must be provider/model")
	}
	return NewModelRef(trimmed[:idx], trimmed[idx+1:])
}

// ProviderID reports the namespace segment.
func (r ModelRef) ProviderID() string { return r.providerID }

// ModelID reports the model identifier inside the provider.
func (r ModelRef) ModelID() string { return r.modelID }

// String renders the wire form "provider/model".
func (r ModelRef) String() string {
	if r.IsZero() {
		return ""
	}
	return r.providerID + "/" + r.modelID
}

// IsZero reports whether the reference holds no pair, which is what an absent
// row decodes to.
func (r ModelRef) IsZero() bool { return r.providerID == "" || r.modelID == "" }

// ModelCapabilities is the set of extra operations a model declares, in
// canonical order and without duplicates. The registry declares them as an open
// list ("text2img", "edit"), so membership is by name rather than a closed enum.
type ModelCapabilities []string

// NewModelCapabilities trims, drops empties, de-duplicates case-insensitively,
// and sorts, so two sets with the same members compare equal and a stored jsonb
// value round-trips to the same bytes.
func NewModelCapabilities(names ...string) ModelCapabilities {
	seen := make(map[string]struct{}, len(names))
	out := make(ModelCapabilities, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

// Has reports whether the set carries a capability, ignoring case so a wire
// value and a registry value cannot disagree over spelling.
func (c ModelCapabilities) Has(name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, item := range c {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

// List returns a copy, so a caller cannot reorder the value object's own set.
func (c ModelCapabilities) List() []string {
	out := make([]string, len(c))
	copy(out, c)
	return out
}

// validateCatalogText normalizes a short display field.
func validateCatalogText(field, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", NewValidationError(field + " is required")
	}
	if len(trimmed) > 120 {
		return "", NewValidationError(field + " must be at most 120 characters")
	}
	if hasControlChars(trimmed) {
		return "", NewValidationError(field + " must not contain control characters")
	}
	return trimmed, nil
}

// validateComboRef accepts the three shapes a combo model ref, an alias
// target, or a judge model may take (SPEC-API-001 §7.7): a provider/model
// reference, a combo name, or an alias. Whether the last two resolve to
// anything is a cross-aggregate question, so the service checks it before the
// write; shape is checked here.
func validateComboRef(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", NewValidationError("model reference is required")
	}
	if hasWhitespace(trimmed) {
		return "", NewValidationError("model reference must not contain whitespace")
	}
	if len(trimmed) > modelSegmentMax {
		return "", NewValidationError("model reference is too long")
	}
	if strings.Contains(trimmed, "/") {
		if _, err := ParseModelRef(trimmed); err != nil {
			return "", err
		}
		return trimmed, nil
	}
	if !isComboName(trimmed) {
		return "", NewValidationError("model reference must be provider/model, a combo name, or an alias")
	}
	return trimmed, nil
}

// hasWhitespace reports whether a value contains any whitespace. Model strings
// and aliases are typed by a human and parsed by a router, so internal
// whitespace is rejected rather than escaped (SPEC-UI §583).
func hasWhitespace(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// hasControlChars reports whether a value carries C0 controls or DEL, which the
// panel strips from every label it accepts (SPEC-UI §583).
func hasControlChars(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
