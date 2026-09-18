// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/embeddings.go
// @for       The OpenAI embeddings request and response contracts.
// @uses      bytes, encoding/json, internal/domain.
// @reason    SPEC-API-001 §7.10 lists POST /api/v1/embeddings as P1 and §7.15
//
//	serves it on the OpenAI wire; AGENTS.md §2.4 requires the typed
//	contract before the handler. Two of its fields are genuine unions on
//	the wire (an input that is text or tokens, a vector that is floats or
//	base64), so both are resolved once here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-17
package schema

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// EmbeddingsRequest is the body of POST /api/v1/embeddings.
type EmbeddingsRequest struct {
	Model          string         `json:"model" validate:"required,min=1,max=200"`
	Input          EmbeddingInput `json:"input"`
	EncodingFormat string         `json:"encoding_format,omitempty" validate:"omitempty,oneof=float base64"`
	Dimensions     *int           `json:"dimensions,omitempty" validate:"omitempty,gte=1"`
	User           string         `json:"user,omitempty"`
}

// EmbeddingInput is the `input` field: a single string, an array of strings, or an
// array of token arrays. All three are kept distinct rather than flattened,
// because a tokenized input is sent to an upstream differently from text.
type EmbeddingInput struct {
	// Texts is the input when it was a string or an array of strings.
	Texts []string
	// Tokens is the input when it was an array of token arrays.
	Tokens [][]int
	// single records that the input arrived as a bare string, so a round trip
	// through this type emits the same shape it received.
	single bool
}

// UnmarshalJSON accepts all three permitted shapes of the input field.
func (i *EmbeddingInput) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return domain.NewValidationError("input must be a string, an array of strings, or an array of tokens")
		}
		i.Texts = []string{text}
		i.single = true
		return nil
	}

	// An array is strings or token arrays, decided by its first element: decoding
	// into the wrong one fails the whole request, and a mixed array is not a shape
	// the wire defines.
	var elements []json.RawMessage
	if err := json.Unmarshal(trimmed, &elements); err != nil {
		return domain.NewValidationError("input must be a string, an array of strings, or an array of tokens")
	}
	if len(elements) == 0 {
		return domain.NewValidationError("input must not be an empty array")
	}
	first := bytes.TrimSpace(elements[0])
	if len(first) > 0 && first[0] == '[' {
		var tokens [][]int
		if err := json.Unmarshal(trimmed, &tokens); err != nil {
			return domain.NewValidationError("input must be a string, an array of strings, or an array of tokens")
		}
		i.Tokens = tokens
		return nil
	}
	var texts []string
	if err := json.Unmarshal(trimmed, &texts); err != nil {
		return domain.NewValidationError("input must be a string, an array of strings, or an array of tokens")
	}
	i.Texts = texts
	return nil
}

// MarshalJSON writes the input back in the shape it arrived in, so a same-format
// upstream receives the request the client sent.
func (i EmbeddingInput) MarshalJSON() ([]byte, error) {
	if len(i.Tokens) > 0 {
		return json.Marshal(i.Tokens)
	}
	if i.single && len(i.Texts) == 1 {
		return json.Marshal(i.Texts[0])
	}
	return json.Marshal(i.Texts)
}

// Single reports whether the input arrived as a bare string.
func (i EmbeddingInput) Single() bool { return i.single }

// IsEmpty reports whether the input carries nothing, which is a validation
// failure rather than an empty embeddings request.
func (i EmbeddingInput) IsEmpty() bool { return len(i.Texts) == 0 && len(i.Tokens) == 0 }

// Count reports how many inputs the request carries, which is what a caller
// checks the response against.
func (i EmbeddingInput) Count() int {
	if len(i.Tokens) > 0 {
		return len(i.Tokens)
	}
	return len(i.Texts)
}

// EmbeddingVector is one embedding: floats in the default encoding, or the base64
// form a caller asked for. Keeping both shapes typed means the response is
// marshalled without an untyped value.
type EmbeddingVector struct {
	// Floats is the vector when the encoding is float.
	Floats []float64
	// Encoded is the base64 payload when the encoding is base64.
	Encoded string
}

// NewFloatVector builds a float-encoded embedding.
func NewFloatVector(values []float64) EmbeddingVector {
	return EmbeddingVector{Floats: values}
}

// NewEncodedVector builds a base64-encoded embedding from raw little-endian
// float32 bytes, which is the representation OpenAI's base64 encoding uses.
func NewEncodedVector(values []float64) EmbeddingVector {
	payload := make([]byte, 0, len(values)*4)
	for _, value := range values {
		bits := math.Float32bits(float32(value))
		payload = append(payload, byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24))
	}
	return EmbeddingVector{Encoded: base64.StdEncoding.EncodeToString(payload)}
}

// MarshalJSON emits whichever encoding the vector carries.
func (v EmbeddingVector) MarshalJSON() ([]byte, error) {
	if v.Encoded != "" {
		return json.Marshal(v.Encoded)
	}
	if v.Floats == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(v.Floats)
}

// EmbeddingObject is one entry of the embeddings response.
type EmbeddingObject struct {
	Object    string          `json:"object"`
	Index     int             `json:"index"`
	Embedding EmbeddingVector `json:"embedding"`
}

// EmbeddingsResponse is the OpenAI embeddings answer.
type EmbeddingsResponse struct {
	Object string            `json:"object"`
	Data   []EmbeddingObject `json:"data"`
	Model  string            `json:"model"`
	Usage  *Usage            `json:"usage,omitempty"`
}

// DecodeEmbeddingsRequest decodes an embeddings body into its typed contract.
func DecodeEmbeddingsRequest(raw []byte) (EmbeddingsRequest, error) {
	var req EmbeddingsRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return EmbeddingsRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
