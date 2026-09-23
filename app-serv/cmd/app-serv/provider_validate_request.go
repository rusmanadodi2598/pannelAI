// Command app-serv shapes the requests a credential check sends.
//
// @file      cmd/app-serv/provider_validate_request.go
// @for       The request shapes the stateless check needs: the one-token chat
//
//	probe, its model name, a node type's wire format, and the URL a
//	provider is checked against.
//
// @uses      internal/domain, internal/registry, encoding/json, strings.
// @reason    Draft 017 §4.6 ports the reference's chat fallback, and what the
//
//	fallback sends is what makes it useful: an upstream rejects an empty
//	model name before it reads the credential, so a validation that omitted
//	one would report a working key as broken. The validate URL belongs here
//	too because it is derived from the same base URL the requests use
//	(§4.2's second consequence: 18 entries declared one, 19 more yield one).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// chatProbeMessage is one turn of the fallback's prompt. It is a concrete type
// rather than a map because AGENTS.md §1.4 forbids `any` outside a declared decode
// boundary, and the shape is fixed.
type chatProbeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatProbeRequest is the one-token chat request a fallback sends.
type chatProbeRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []chatProbeMessage `json:"messages"`
}

// chatProbeBody encodes the fallback request.
func chatProbeBody(modelID string) []byte {
	encoded, err := json.Marshal(chatProbeRequest{
		Model:     modelID,
		MaxTokens: chatProbeMaxTokens,
		Messages:  []chatProbeMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		// reason: the payload is a fixed literal shape, so encoding it cannot
		// fail; a request with no body would be rejected by the upstream for the
		// wrong reason, which reads as a credential problem.
		return []byte(`{"model":"` + modelID + `","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	}
	return encoded
}

// modelIDOrDefault names the model a chat probe asks for.
func modelIDOrDefault(modelID string) string {
	if trimmed := strings.TrimSpace(modelID); trimmed != "" {
		return trimmed
	}
	return probeModelFallback
}

// nodeFormat maps a node type onto the wire format the connector places a
// credential for.
func nodeFormat(check service.CredentialCheck) string {
	if check.NodeType == string(domain.NodeAnthropicCompatible) {
		return "claude"
	}
	if strings.EqualFold(check.APIType, domain.NodeAPIResponses) {
		return registry.FormatOpenAIResponses
	}
	return registry.DefaultFormat
}

// validateTarget is the URL a provider's credential is checked against.
//
// A declared `transport.validate_url` wins. Otherwise it is derived from the
// base URL by the reference's own rule (providers/validate/route.js:626-627):
// replace a trailing `/chat/completions` or `/chatbot` with `/models`. That is
// what takes probe coverage from the 18 entries that declare a URL to every
// provider whose base names a chat path (draft 017 §4.2's second consequence).
//
// A base that names neither returns empty, and the caller reports the reason
// rather than guessing a path — a guess produces a 404 that reads like a
// credential problem.
func validateTarget(entry registry.Provider) string {
	if declared := strings.TrimSpace(entry.Transport.ValidateURL); declared != "" {
		return declared
	}
	base := strings.TrimSpace(entry.Transport.BaseURL)
	if base == "" {
		return ""
	}
	derived := strings.TrimSuffix(base, "/chat/completions")
	if derived == base {
		derived = strings.TrimSuffix(base, "/chatbot")
	}
	if derived == base {
		return ""
	}
	return derived + "/models"
}
