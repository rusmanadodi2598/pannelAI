// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/suffix.go
// @for       The "(level)" suffix a client may append to a model string.
// @uses      regexp, strconv, strings.
// @reason    SPEC-API-001 §7.15 ports the reference's per-model control: the
//
//	panel's reasoning select appends the suffix to copied model names, and
//	the gateway must strip it before resolving the model and read it as an
//	override that outranks the stored provider mode. Keeping the parse
//	here, next to the maps it validates against, is what stops the
//	resolver and the injection from disagreeing about the same string.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import (
	"regexp"
	"strconv"
	"strings"
)

// Suffix is the reasoning override a model string carried. Mode is one of
// "none" (thinking off), "auto" (the upstream's own default), "level" (a named
// level in Level), or "budget" (a token count in Budget).
type Suffix struct {
	Mode   string
	Level  string
	Budget int
}

// suffixPattern is the reference's own expression: a trailing parenthesised
// group with no nesting, so a model id that legitimately contains parentheses
// elsewhere is untouched. The leading group is greedy, so the LAST group is the
// one read.
var suffixPattern = regexp.MustCompile(`^(.*)\([^()]+\)\s*$`)

// suffixValuePattern captures the same shape with the value separated.
var suffixValuePattern = regexp.MustCompile(`^(.*)\(([^()]+)\)\s*$`)

// StripSuffix removes a trailing "(value)" group from a model string, which is
// what the resolver and the upstream model id must see. A string without one is
// returned unchanged.
func StripSuffix(model string) string {
	match := suffixPattern.FindStringSubmatch(model)
	if match == nil {
		return model
	}
	return strings.TrimSpace(match[1])
}

// ParseSuffix splits a model string into the clean id and the override it
// carries. A value the vocabulary does not know leaves the override nil while
// still stripping the group, exactly as the reference does: the string was a
// suffix by shape, and the upstream must not receive it.
func ParseSuffix(model string) (string, *Suffix) {
	match := suffixValuePattern.FindStringSubmatch(model)
	if match == nil {
		return model, nil
	}
	clean := strings.TrimSpace(match[1])
	raw := strings.ToLower(strings.TrimSpace(match[2]))

	switch {
	case raw == "none" || raw == "off":
		return clean, &Suffix{Mode: "none"}
	case raw == "auto":
		return clean, &Suffix{Mode: "auto"}
	case raw == "ultra":
		return clean, &Suffix{Mode: "level", Level: raw}
	case isDigits(raw):
		budget, err := strconv.Atoi(raw)
		if err != nil {
			return clean, nil
		}
		return clean, &Suffix{Mode: "budget", Budget: budget}
	default:
		if _, known := LevelToBudget[raw]; known {
			return clean, &Suffix{Mode: "level", Level: raw}
		}
		return clean, nil
	}
}

// isDigits reports whether the value is a non-negative integer, the reference's
// `/^\d+$/` test. A value that long overflows strconv, which ParseSuffix reads
// as "not a budget" rather than as a number nothing could send.
func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
