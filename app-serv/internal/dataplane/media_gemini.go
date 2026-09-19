// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/media_gemini.go
// @for       The Gemini embeddings payload shape: its URL, its typed request
//
//	bodies, and the single-versus-batch split.
//
// @uses      internal/registry, encoding/json, strings.
// @reason    Gemini addresses embeddings by model-qualified operation URLs and
//
//	wraps batch inputs in a `requests` envelope, which is vendor
//	shape rather than the §8.1 placement rule that media.go owns.
//	Separating the two keeps the per-kind credential placement
//	readable without a vendor dialect inside it, and gives the
//	payload a typed home (AGENTS.md §1.4) instead of a map.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// GeminiEmbeddingFormat marks the Gemini embedding payload shape, which is
// `{model, content: {parts}}` for one input and `{requests: [...]}` for several.
const GeminiEmbeddingFormat = "gemini-embedding"

// IsGeminiEmbedding reports whether a media block declares the Gemini embeddings
// shape, which is decided by its base URL: the registry declares no per-kind format
// for embeddings, and the endpoint is what distinguishes the two protocols.
func IsGeminiEmbedding(media registry.MediaConfig) bool {
	return strings.Contains(media.BaseURL, "generativelanguage.googleapis.com")
}

// GeminiEmbeddingsURL builds the :embedContent or :batchEmbedContents URL, which is
// how Gemini's embeddings API is addressed.
func GeminiEmbeddingsURL(baseURL, upstreamModel string, batch bool) string {
	model := upstreamModel
	if !strings.HasPrefix(model, "models/") {
		model = "models/" + model
	}
	operation := "embedContent"
	if batch {
		operation = "batchEmbedContents"
	}
	return strings.TrimSuffix(baseURL, "/") + "/" + model + ":" + operation
}

// GeminiEmbeddingsBody builds the Gemini embeddings payload.
func GeminiEmbeddingsBody(model string, input GeminiEmbeddingInput) []byte {
	path := model
	if !strings.HasPrefix(path, "models/") {
		path = "models/" + model
	}
	payload := geminiEmbeddingsPayload{
		Model:   path,
		Content: geminiEmbedContent{Parts: []geminiEmbedPart{{Text: input.Single}}},
	}
	if len(input.Many) > 0 {
		payload.Requests = make([]geminiEmbeddingsPayload, 0, len(input.Many))
		for _, text := range input.Many {
			payload.Requests = append(payload.Requests, geminiEmbeddingsPayload{
				Model:   path,
				Content: geminiEmbedContent{Parts: []geminiEmbedPart{{Text: text}}},
			})
		}
		payload.Model = ""
		payload.Content = geminiEmbedContent{}
	}
	if input.Dimensions > 0 {
		payload.OutputDimensionality = input.Dimensions
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return encoded
}

// geminiEmbeddingsPayload is one Gemini embedContent request, or the batch
// envelope when Requests is set (batchEmbedContents carries the per-request
// model and content inside `requests`, with the dimensionality beside them).
type geminiEmbeddingsPayload struct {
	Model                string                    `json:"model,omitempty"`
	Content              geminiEmbedContent        `json:"content,omitempty"`
	Requests             []geminiEmbeddingsPayload `json:"requests,omitempty"`
	OutputDimensionality int                       `json:"outputDimensionality,omitempty"`
}

// geminiEmbedContent is the text part container Gemini expects.
type geminiEmbedContent struct {
	Parts []geminiEmbedPart `json:"parts"`
}

// geminiEmbedPart is one text part.
type geminiEmbedPart struct {
	Text string `json:"text"`
}

// GeminiEmbeddingInput is the Gemini payload's input, kept typed so the body
// builder needs no untyped value.
type GeminiEmbeddingInput struct {
	// Single is the text of a one-input request.
	Single string
	// Many is the text of a multi-input request, which uses batchEmbedContents.
	Many []string
	// Dimensions is the requested output size, or zero for the model's default.
	Dimensions int
}
