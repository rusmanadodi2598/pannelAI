// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/custom_node.go
// @for       Synthesis of a user-defined provider node into a registry entry.
// @uses      internal/registry, fmt, net/url, strings.
// @reason    SPEC-API-001 §7.4 lets an operator define their own
//
//	OpenAI-compatible or Anthropic-compatible base URL. Synthesizing
//	the provider entry here, rather than branching on "is this custom"
//	in every downstream layer, is what keeps a node routable through
//	exactly the same path as a registry provider.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import (
	"fmt"
	"net/url"
	"strings"
)

// ID prefixes for custom provider nodes. They are part of the public contract:
// a node's id appears in endpoint rows and in the panel's URLs, and the loader
// derives the node's wire format from which prefix it carries.
const (
	OpenAICompatiblePrefix    = "openai-compatible-"
	AnthropicCompatiblePrefix = "anthropic-compatible-"
)

// CustomNode is a user-defined provider created through the management API: a
// base URL the embedded registry does not ship (SPEC-API-001 §7.4). It is
// declared here rather than in the domain layer because synthesizing the
// provider entry is what the registry owns.
type CustomNode struct {
	ID      string
	Name    string
	Prefix  string
	APIType string
	BaseURL string
}

// OpenAITypeResponses is the api type an OpenAI-compatible node declares when
// its upstream serves the Responses API rather than chat completions.
const OpenAITypeResponses = "responses"

// format reports which wire format the node speaks. An Anthropic-compatible
// node is identified by its id prefix, and an OpenAI-compatible node by its
// declared api type: `responses` is a different endpoint and a different
// payload from `chat`, so treating both as "openai" would send a Responses
// request to /chat/completions, which cannot serve it.
func (n CustomNode) format() string {
	if strings.HasPrefix(n.ID, AnthropicCompatiblePrefix) {
		return "claude"
	}
	if n.APIType == OpenAITypeResponses {
		return FormatOpenAIResponses
	}
	return DefaultFormat
}

// chatPath is the path that completes the node's base URL. A node stores a
// base ("https://host/v1"), not a full endpoint URL, so the path has to come
// from the node's kind. The reference's BaseExecutor.buildUrl appends exactly
// these three, and the operator's own endpoint row is the only other place the
// convention appears.
func (n CustomNode) chatPath() string {
	switch {
	case strings.HasPrefix(n.ID, AnthropicCompatiblePrefix):
		return "/messages"
	case n.APIType == OpenAITypeResponses:
		return "/responses"
	default:
		return "/chat/completions"
	}
}

// Validate checks the node on its own, without reference to the index. It is
// exported because the management API validates a create request before it
// looks at collisions, so the caller can report the field that is wrong.
func (n CustomNode) Validate() error {
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(n.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(n.Prefix) == "" {
		return fmt.Errorf("prefix is required")
	}
	if !isNodePrefix(n.Prefix) {
		return fmt.Errorf("prefix must contain only letters, digits, dots, dashes, and underscores")
	}
	switch {
	case strings.HasPrefix(n.ID, OpenAICompatiblePrefix):
		if n.APIType != "chat" && n.APIType != "responses" {
			return fmt.Errorf("api_type must be chat or responses for an OpenAI-compatible node")
		}
	case strings.HasPrefix(n.ID, AnthropicCompatiblePrefix):
		if n.APIType != "" {
			return fmt.Errorf("api_type does not apply to an Anthropic-compatible node")
		}
	default:
		return fmt.Errorf("id must start with %q or %q", OpenAICompatiblePrefix, AnthropicCompatiblePrefix)
	}
	return validateBaseURL(n.BaseURL)
}

// isNodePrefix reports whether the prefix can serve as a model-string namespace.
// A prefix containing "/" or whitespace would make "prefix/model" ambiguous or
// unparseable, so it is rejected rather than escaped at parse time.
func isNodePrefix(prefix string) bool {
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// Synthesize builds the provider entry a custom node stands for, so every
// downstream layer sees one provider shape instead of branching on "is this
// custom".
func (i *Index) Synthesize(node CustomNode) (Provider, error) {
	if err := node.Validate(); err != nil {
		return Provider{}, fmt.Errorf("registry: custom node %q: %w", node.ID, err)
	}
	// The prefix becomes a model-string namespace, so it must not shadow a
	// built-in identifier: "mycorp/model" would otherwise resolve to two
	// different providers depending on which table was consulted first.
	if owner, taken := i.byName[node.Prefix]; taken {
		return Provider{}, fmt.Errorf("registry: prefix %q is already used by %q", node.Prefix, owner)
	}

	provider := Provider{
		ID:        node.ID,
		Priority:  CustomPriority,
		Alias:     node.Prefix,
		Category:  "apikey",
		AuthType:  AuthAPIKey,
		Custom:    true,
		Display:   Display{Name: node.Name},
		Transport: Transport{Format: node.format(), BaseURL: node.BaseURL, ChatPath: node.chatPath()},
	}
	return provider, nil
}

// validateBaseURL requires an absolute http(s) URL. A relative or scheme-less
// value would be joined onto nothing at call time and fail as a confusing
// transport error rather than as the configuration mistake it is.
func validateBaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("base_url is not a valid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base_url must be an absolute http or https URL")
	}
	if parsed.Host == "" {
		return fmt.Errorf("base_url must name a host")
	}
	return nil
}

// WithCustom returns a new Index that also resolves the given custom nodes. The
// receiver is unchanged, so the boot-time index stays immutable and one request
// adding a node cannot affect another request's view.
func (i *Index) WithCustom(nodes ...CustomNode) (*Index, error) {
	doc := Document{Revision: i.revision, Providers: make([]Provider, 0, len(i.providers)+len(nodes))}
	doc.Providers = append(doc.Providers, i.providers...)
	for _, node := range nodes {
		provider, err := i.Synthesize(node)
		if err != nil {
			return nil, err
		}
		doc.Providers = append(doc.Providers, provider)
	}
	return NewIndex(doc)
}
