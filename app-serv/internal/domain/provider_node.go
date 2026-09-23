// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node.go
// @for       The ProviderNode aggregate root: a user-defined OpenAI-compatible
//
//	or Anthropic-compatible upstream (SPEC-API-001 §7.4).
//
// @uses      internal/domain (ULID, error constructors), strings.
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
	baseURL, err := normalizeNodeBaseURL(baseURL, nodeType, apiType)
	if err != nil {
		return ProviderNode{}, err
	}
	id = nodeID(id, nodeType, now)
	return ProviderNode{
		id:        id,
		nodeType:  nodeType,
		name:      name,
		prefix:    prefix,
		apiType:   apiType,
		baseURL:   baseURL,
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
	normalized, err := normalizeNodeBaseURL(baseURL, n.nodeType, n.apiType)
	if err != nil {
		return err
	}
	n.baseURL = normalized
	n.updatedAt = now
	return nil
}
