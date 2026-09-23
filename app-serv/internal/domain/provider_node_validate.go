// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node_validate.go
// @for       The ProviderNode field rules: the prefix namespace, the per-type
//
//	api_type rule, and the id shape.
//
// @uses      internal/domain (ULID, error constructors), strings, time.
// @reason    A prefix is a model-string namespace and an api type picks the
//
//	endpoint the gateway calls, so each rule spans a value's meaning
//	rather than a field's shape — which is why they live in the domain
//	rather than as struct tags. Split from the aggregate to keep both
//	files under AGENTS.md §1.1.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"strings"
	"time"
)

// validateNodePrefix enforces that a prefix can serve as a model namespace.
// A prefix containing "/" or whitespace would make "prefix/model" ambiguous or
// unparseable, so it is rejected rather than escaped at parse time.
func validateNodePrefix(prefix string) error {
	if prefix == "" {
		return NewValidationError("prefix is required")
	}
	if len(prefix) > 64 {
		return NewValidationError("prefix must be at most 64 characters")
	}
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return NewValidationError("prefix must contain only letters, digits, dots, dashes, and underscores")
		}
	}
	return nil
}

// nodeID returns the node's id: a fresh one when the caller supplied none, and
// the caller's id normalized to carry its type prefix otherwise.
//
// Normalizing rather than refusing follows NewCustomModel's precedent: the
// prefix is this package's vocabulary, and a caller that passes a bare ULID
// should not have to know it. The invariant is the id's shape, because the
// registry reads the wire format out of it (SPEC-API-001 §7.4).
func nodeID(id string, nodeType NodeType, now time.Time) string {
	prefix := NodeIDPrefixOpenAI
	if nodeType == NodeAnthropicCompatible {
		prefix = NodeIDPrefixAnthropic
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return prefix + NewULID(now)
	}
	if strings.HasPrefix(id, prefix) {
		return id
	}
	return prefix + id
}

// validateNodeAPIType enforces the per-type rule: an OpenAI-compatible node must
// declare chat or responses, and an Anthropic-compatible node must not, because
// its single endpoint has no such distinction.
func validateNodeAPIType(nodeType NodeType, apiType string) error {
	switch nodeType {
	case NodeOpenAICompatible:
		if apiType != NodeAPIChat && apiType != NodeAPIResponses {
			return NewValidationError("api_type must be chat or responses for an OpenAI-compatible node")
		}
	case NodeAnthropicCompatible:
		if apiType != "" {
			return NewValidationError("api_type does not apply to an Anthropic-compatible node")
		}
	default:
		return NewValidationError("invalid type: " + string(nodeType))
	}
	return nil
}
