// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node.go
// @for       The ProviderNode aggregate root: a user-defined OpenAI-compatible
//
//	or Anthropic-compatible upstream (SPEC-API-001 §7.4).
//
// @uses      internal/domain (ULID, error constructors), net/url, strings.
// @reason    A node is not an endpoint: it has no credential of its own, and
//
//	its prefix becomes a model-string namespace. That namespace rule is
//	the reason this is an aggregate with its own invariants rather than
//	a row the endpoint service happens to read.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-17
package domain

import (
	"net/url"
	"strings"
	"time"
)

// NodeType is how a custom node speaks to its upstream. The set is closed
// because each type maps to one wire format the gateway can translate.
type NodeType string

const (
	NodeOpenAICompatible    NodeType = "openai-compatible"
	NodeAnthropicCompatible NodeType = "anthropic-compatible"
)

// Node API types. An OpenAI-compatible upstream serves either chat completions
// or the Responses API, and they are different endpoints with different
// payloads, so the choice is part of the node's identity rather than a runtime
// guess.
const (
	NodeAPIChat      = "chat"
	NodeAPIResponses = "responses"
)

// ProviderNode is a user-defined provider definition. Fields are unexported on
// purpose (AGENTS.md §2.2): every change goes through a method here, so the
// prefix and base URL invariants cannot be bypassed.
type ProviderNode struct {
	id        string
	nodeType  NodeType
	name      string
	prefix    string
	apiType   string
	baseURL   string
	createdAt time.Time
	updatedAt time.Time
}

// NewProviderNode is the only constructor for a new node.
//
// Validation happens here rather than only in the API layer because the prefix
// is a routing key: a namespace that collides or cannot be parsed makes a model
// string ambiguous, and that is a domain rule, not a request-shape rule. The
// type is validated before the id is minted, because the id's prefix comes from
// the type: minting first would let an unknown type produce an id no layer can
// classify.
func NewProviderNode(id, name, prefix string, nodeType NodeType, apiType, baseURL string, now time.Time) (ProviderNode, error) {
	name = strings.TrimSpace(name)
	prefix = strings.TrimSpace(prefix)
	if name == "" {
		return ProviderNode{}, NewValidationError("name is required")
	}
	if len(name) > 120 {
		return ProviderNode{}, NewValidationError("name must be at most 120 characters")
	}
	if err := validateNodePrefix(prefix); err != nil {
		return ProviderNode{}, err
	}
	if err := validateNodeAPIType(nodeType, apiType); err != nil {
		return ProviderNode{}, err
	}
	if err := validateNodeBaseURL(baseURL); err != nil {
		return ProviderNode{}, err
	}
	id = nodeID(id, nodeType, now)
	return ProviderNode{
		id:        id,
		nodeType:  nodeType,
		name:      name,
		prefix:    prefix,
		apiType:   apiType,
		baseURL:   strings.TrimSpace(baseURL),
		createdAt: now,
		updatedAt: now,
	}, nil
}

// RehydrateProviderNode rebuilds a stored row. For the repository load path
// only; never use it to create a node.
func RehydrateProviderNode(id string, nodeType NodeType, name, prefix, apiType, baseURL string, createdAt, updatedAt time.Time) ProviderNode {
	return ProviderNode{
		id: id, nodeType: nodeType, name: name, prefix: prefix,
		apiType: apiType, baseURL: baseURL, createdAt: createdAt, updatedAt: updatedAt,
	}
}

// Accessors expose state without allowing mutation.
func (n ProviderNode) ID() string           { return n.id }
func (n ProviderNode) Type() NodeType       { return n.nodeType }
func (n ProviderNode) Name() string         { return n.name }
func (n ProviderNode) Prefix() string       { return n.prefix }
func (n ProviderNode) APIType() string      { return n.apiType }
func (n ProviderNode) BaseURL() string      { return n.baseURL }
func (n ProviderNode) CreatedAt() time.Time { return n.createdAt }
func (n ProviderNode) UpdatedAt() time.Time { return n.updatedAt }

// Format reports the wire format this node's upstream speaks, which is what
// makes it routable through the same path as a registry provider.
func (n ProviderNode) Format() string {
	if n.nodeType == NodeAnthropicCompatible {
		return "claude"
	}
	if n.apiType == NodeAPIResponses {
		return "openai-responses"
	}
	return "openai"
}

// Rename changes the display name.
func (n *ProviderNode) Rename(name string, now time.Time) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewValidationError("name is required")
	}
	if len(name) > 120 {
		return NewValidationError("name must be at most 120 characters")
	}
	n.name = name
	n.updatedAt = now
	return nil
}

// Reprefix changes the model-string namespace. A caller that owns an index must
// re-check collisions; this method enforces only the local shape rule.
func (n *ProviderNode) Reprefix(prefix string, now time.Time) error {
	prefix = strings.TrimSpace(prefix)
	if err := validateNodePrefix(prefix); err != nil {
		return err
	}
	n.prefix = prefix
	n.updatedAt = now
	return nil
}

// Rebase changes the upstream base URL.
func (n *ProviderNode) Rebase(baseURL string, now time.Time) error {
	if err := validateNodeBaseURL(baseURL); err != nil {
		return err
	}
	n.baseURL = strings.TrimSpace(baseURL)
	n.updatedAt = now
	return nil
}

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

// validateNodeBaseURL requires an absolute http(s) URL. A relative or
// scheme-less value would be joined onto nothing at call time and fail as a
// confusing transport error rather than as the configuration mistake it is.
func validateNodeBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return NewValidationError("base_url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return NewValidationError("base_url is not a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return NewValidationError("base_url must be an absolute http or https URL")
	}
	if parsed.Host == "" {
		return NewValidationError("base_url must name a host")
	}
	return nil
}
