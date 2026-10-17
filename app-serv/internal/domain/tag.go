// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/tag.go
// @for       The Tag value object and the set rules one row's tags obey.
// @uses      strings, internal/domain (error constructors).
// @reason    A tag is the address a bulk action is sent to, so two spellings of one label would split the set it names, and an unbounded set would make the containment index read like a scan. Both rules are properties of the value, so they are enforced here rather than at each caller.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import "strings"

const (
	// MaxTagsPerRow bounds one row's tag set. A row carrying hundreds of labels is a
	// query by another name, and the aggregate refuses rather than storing a set the
	// containment index would have to read in full.
	MaxTagsPerRow = 16
	// MaxTagLength is the width of the pattern below: one character plus 47 more.
	MaxTagLength = 48
)

// Tag is one operator's label over an account or a key. It is stored and compared in
// its canonical spelling, which NewTag produces: lowercase and trimmed.
type Tag string

// NewTag builds one tag from whatever the caller wrote. An over-long or malformed tag
// is refused rather than cut, so a bulk action never silently lands on fewer rows than
// the operator named.
func NewTag(raw string) (Tag, error) {
	tag := strings.ToLower(strings.TrimSpace(raw))
	if tag == "" {
		return "", NewValidationError("tag is required")
	}
	if len(tag) > MaxTagLength {
		return "", NewValidationError("tag must be at most 48 characters")
	}
	// The shape is [a-z0-9] then [a-z0-9_-], checked without a regexp because the
	// alphabet is five classes and a loop names them.
	for i := 0; i < len(tag); i++ {
		c := tag[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case (c == '-' || c == '_') && i > 0:
		default:
			return "", NewValidationError("tag must start with a letter or digit and hold only lowercase letters, digits, dashes, and underscores")
		}
	}
	return Tag(tag), nil
}

// String returns the canonical spelling the row stores.
func (t Tag) String() string { return string(t) }

// NormalizeTags turns caller-supplied strings into the set a row stores: each tag
// parsed, duplicates folded, and the bound applied to the folded count so a label
// written twice does not spend two slots.
func NormalizeTags(raw []string) ([]Tag, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := make(map[Tag]struct{}, len(raw))
	out := make([]Tag, 0, len(raw))
	for _, candidate := range raw {
		tag, err := NewTag(candidate)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[tag]; dup {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) > MaxTagsPerRow {
		return nil, NewValidationError("a row may hold at most 16 tags")
	}
	return out, nil
}
