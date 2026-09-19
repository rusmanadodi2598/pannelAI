// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_node.go
// @for       The embeddings block synthesized for a custom OpenAI-compatible
//
//	node, which declares none of its own.
//
// @uses      internal/registry, strings.
// @reason    SPEC-API-001 §7.4 lets an operator add an OpenAI-compatible node,
//
//	and the reference serves embeddings for exactly those nodes through
//	a dedicated adapter (`openaiCompatNode.js`). Without this branch a
//	node that answers chat is refused on the embeddings route with
//	"does not offer embeddings", which is a parity gap rather than a
//	missing provider feature.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// nodeEmbeddingMedia synthesizes the embeddings block a custom node does not
// declare, from the node's own base URL.
//
// Only an OpenAI-compatible node gets one: an Anthropic-compatible node's
// upstream speaks the Messages wire format and serves no embeddings endpoint,
// so its refusal stays. The declared Format keeps the payload OpenAI-shaped
// even when the operator points the node at a host that also serves a native
// protocol, which the URL heuristic in dataplane.IsGeminiEmbedding must not
// misread here.
func nodeEmbeddingMedia(entry registry.Provider) (registry.MediaConfig, bool) {
	if !entry.Custom {
		return registry.MediaConfig{}, false
	}
	switch entry.Transport.Format {
	case registry.DefaultFormat, registry.FormatOpenAIResponses:
	default:
		return registry.MediaConfig{}, false
	}
	base := nodeEmbeddingsURL(entry.Transport.BaseURL)
	if base == "" {
		return registry.MediaConfig{}, false
	}
	return registry.MediaConfig{
		BaseURL:  base,
		AuthType: registry.AuthAPIKey,
		Format:   registry.DefaultFormat,
	}, true
}

// nodeEmbeddingsURL completes a node's base URL the way the reference's
// openaiCompatNode adapter does: a trailing slash and an already-named
// `/embeddings` are trimmed before the path is appended, so both
// "https://host/v1" and "https://host/v1/embeddings" end at one URL rather
// than a doubled path.
func nodeEmbeddingsURL(base string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(base), "/")
	trimmed = strings.TrimSuffix(trimmed, "/embeddings")
	if trimmed == "" {
		return ""
	}
	return trimmed + "/embeddings"
}
