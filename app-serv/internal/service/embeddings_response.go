// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_response.go
// @for       The answer half of the embeddings use case: normalizing either wire
//
//	shape into the OpenAI embeddings response the client expects.
//
// @uses      internal/schema, encoding/json.
// @reason    The request half (media-config resolution, credential placement) and
//
//	the answer half (shape normalization) change for different
//	reasons: the request follows §8.1's placement rules, the answer
//	follows what each upstream sends back. Keeping the OpenAI
//	normalization and the Gemini conversion beside each other here is
//	what makes the response contract auditable in one place, and keeps
//	the use case file within the §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-18
package service

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// A body already in that shape is passed through field by field, because the
// response carries vectors whose length the gateway must not reinterpret; a
// provider-native shape (Gemini's embeddings array) is converted.
func normalizeEmbeddings(raw []byte, req schema.EmbeddingsRequest, model string, gemini bool) schema.EmbeddingsResponse {
	if !gemini {
		var shaped struct {
			Object string `json:"object"`
			Data   []struct {
				Object    string          `json:"object"`
				Index     int             `json:"index"`
				Embedding json.RawMessage `json:"embedding"`
			} `json:"data"`
			Model string        `json:"model"`
			Usage *schema.Usage `json:"usage"`
		}
		if err := json.Unmarshal(raw, &shaped); err == nil && len(shaped.Data) > 0 {
			response := schema.EmbeddingsResponse{
				Object: "list", Data: make([]schema.EmbeddingObject, 0, len(shaped.Data)),
				Model: shaped.Model, Usage: shaped.Usage,
			}
			if response.Model == "" {
				response.Model = model
			}
			for _, item := range shaped.Data {
				response.Data = append(response.Data, schema.EmbeddingObject{
					Object: "embedding", Index: item.Index,
					Embedding: decodeVector(item.Embedding, req.EncodingFormat),
				})
			}
			return response
		}
	}
	return geminiEmbeddingsFromNative(raw, model, req.EncodingFormat)
}

// decodeVector reads one embedding in whichever encoding the upstream sent it.
func decodeVector(raw json.RawMessage, encoding string) schema.EmbeddingVector {
	var floats []float64
	if err := json.Unmarshal(raw, &floats); err == nil {
		if encoding == "base64" {
			return schema.NewEncodedVector(floats)
		}
		return schema.NewFloatVector(floats)
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		return schema.EmbeddingVector{Encoded: encoded}
	}
	return schema.NewFloatVector(nil)
}

// geminiEmbeddingsFromNative converts Gemini's embeddings array into the OpenAI
// list shape, numbering each entry in arrival order.
func geminiEmbeddingsFromNative(raw []byte, model, encoding string) schema.EmbeddingsResponse {
	var native struct {
		Embeddings []struct {
			Values []float64 `json:"values"`
		} `json:"embeddings"`
		Embedding struct {
			Values []float64 `json:"values"`
		} `json:"embedding"`
	}
	response := schema.EmbeddingsResponse{Object: "list", Data: make([]schema.EmbeddingObject, 0, 1), Model: model}
	if err := json.Unmarshal(raw, &native); err != nil {
		return response
	}
	appendVector := func(index int, values []float64) {
		vector := schema.NewFloatVector(values)
		if encoding == "base64" {
			vector = schema.NewEncodedVector(values)
		}
		response.Data = append(response.Data, schema.EmbeddingObject{
			Object: "embedding", Index: index, Embedding: vector,
		})
	}
	for index, item := range native.Embeddings {
		appendVector(index, item.Values)
	}
	if len(response.Data) == 0 && native.Embedding.Values != nil {
		appendVector(0, native.Embedding.Values)
	}
	return response
}

// upstreamMessageOf extracts a message from an upstream error body, so the client
// gets something actionable rather than an empty string.
func upstreamMessageOf(body []byte) string {
	var shaped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &shaped) == nil {
		if shaped.Error.Message != "" {
			return shaped.Error.Message
		}
		if shaped.Message != "" {
			return shaped.Message
		}
	}
	return "the upstream rejected the request"
}
