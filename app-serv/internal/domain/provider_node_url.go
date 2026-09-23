// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/provider_node_url.go
// @for       Validation and normalization of a provider node's base URL: the
//
//	absolute-http(s) rule, and the rule that a stored base carries
//	neither a trailing slash nor the path the transport appends.
//
// @uses      net/url, strings.
// @reason    A node stores a base ("https://host/v1"), not a full endpoint, and
//
//	the transport appends the path its type speaks. An operator who
//	pastes the endpoint URL from a vendor's documentation therefore
//	stores a base that already carries that path, and the call goes to
//	a doubled path. The rule is one function here rather than a branch
//	in each write path because the reference sanitized create and update
//	separately, and that separation is exactly how the two drift.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-23
package domain

import (
	"net/url"
	"strings"
)

// nodeAppendedSuffix reports the path the transport appends to this node's base
// URL, or "" when the node appends nothing.
//
// It is derived from the node's type and api type rather than passed in, so a
// caller cannot sanitize against a path the transport would not use.
func nodeAppendedSuffix(nodeType NodeType, apiType string) string {
	switch nodeType {
	case NodeAnthropicCompatible:
		return "/messages"
	case NodeOpenAICompatible:
		if apiType == NodeAPIResponses {
			return "/responses"
		}
		return "/chat/completions"
	default:
		return ""
	}
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

// normalizeNodeBaseURL validates a base URL and returns the form the node
// should store: no trailing slash, and no trailing copy of the path the
// transport appends.
//
// Both halves are applied to the URL's PATH, never to the raw string. That is
// what keeps a host that happens to spell the suffix — "https://messages" —
// from losing its host, and it is why a query string survives untouched.
//
// The suffix is stripped repeatedly rather than once, so an operator who pasted
// an already-doubled URL ("…/v1/messages/messages") is repaired to "…/v1"
// instead of to another URL that is still wrong.
func normalizeNodeBaseURL(raw string, nodeType NodeType, apiType string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if err := validateNodeBaseURL(trimmed); err != nil {
		return "", err
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		// validateNodeBaseURL already parsed this successfully, so an error
		// here is unreachable; returning it keeps the function total rather
		// than panicking on a value a future edit might route around.
		return "", NewValidationError("base_url is not a valid URL")
	}

	suffix := nodeAppendedSuffix(nodeType, apiType)
	for {
		trimmedPath := strings.TrimRight(parsed.Path, "/")
		// A path that is empty after trimming is the host alone; there is
		// nothing left that could be the appended suffix.
		if trimmedPath == "" {
			parsed.Path = ""
			break
		}
		// Only a whole final segment matches. "/messages-extra" ends with the
		// suffix as a substring but is a different path, and stripping it would
		// rewrite a URL the operator got right.
		if suffix != "" && strings.HasSuffix(trimmedPath, suffix) {
			trimmedPath = strings.TrimSuffix(trimmedPath, suffix)
			parsed.Path = trimmedPath
			continue
		}
		parsed.Path = trimmedPath
		break
	}
	return parsed.String(), nil
}
