// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings.go
// @for       The embeddings data plane use case: media-config resolution, the
//
//	per-kind credential placement, and the OpenAI-shape normalization.
//
// @uses      internal/dataplane, internal/domain, internal/registry, internal/schema.
// @reason    SPEC-API-001 §8.1 puts the credential placement in the kind's own
//
//	block because `auth_header: key` is a QUERY PARAMETER: Gemini's
//	embedding config uses one while its chat transport does not. Reusing
//	the chat transport would authenticate incorrectly rather than fail
//	loudly, so the media path is its own use case, and §7.10 forbids a
//	silent cloud fallback when a base URL is missing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// EmbeddingsService implements the embeddings half of SPEC-API-001 §7.10.
type EmbeddingsService struct {
	resolver  ModelResolver
	router    MediaRouter
	caller    dataplane.MediaCaller
	overrides MediaOverrideReader
	recorder  dataPlaneRecorder
	active    dataplane.ActiveRequests
}

// EmbeddingsServiceDeps holds the collaborators the service needs.
type EmbeddingsServiceDeps struct {
	// Resolver and Router are the two questions the use case asks of the
	// engine, as narrow ports: the wire translators stay out of every test
	// that only needs a model resolved or an account picked.
	Resolver ModelResolver
	Router   MediaRouter
	Caller   dataplane.MediaCaller
	// Overrides reads the stored per-provider base URL §7.10's save writes.
	// It is optional so a deployment that has not configured one still
	// resolves the registry's own base URL.
	Overrides MediaOverrideReader
	// Usage and Logs write the §7.12/§7.13 accounting pair for every call that
	// reaches an upstream; Quotas advances the §7.12 window counters; RequestID
	// gives both rows one identifier. All four are optional so a deployment that
	// wires none still serves.
	Usage     UsageRecorder
	Logs      RequestLogRecorder
	Quotas    *QuotaCounter
	RequestID RequestIDReader
	// ActiveRequests marks one provider as in flight while its call runs, which
	// is what the §7.12 live stream draws. Optional for the same reason as the
	// other three: a deployment that wires none still serves.
	ActiveRequests dataplane.ActiveRequests
}

// NewEmbeddingsService validates deps and returns a ready service.
func NewEmbeddingsService(deps EmbeddingsServiceDeps) (*EmbeddingsService, error) {
	if deps.Resolver == nil {
		return nil, domain.NewValidationError("model resolver is required")
	}
	if deps.Router == nil {
		return nil, domain.NewValidationError("media router is required")
	}
	if deps.Caller == nil {
		return nil, domain.NewValidationError("media caller is required")
	}
	return &EmbeddingsService{
		resolver: deps.Resolver, router: deps.Router, caller: deps.Caller, overrides: deps.Overrides,
		recorder: newDataPlaneRecorder(deps.Usage, deps.Logs, deps.Quotas, deps.RequestID),
		active:   deps.ActiveRequests,
	}, nil
}

// Embed resolves the model, selects the account, performs the call, and returns the
// answer in the OpenAI embeddings shape.
//
// Resolution runs through the same resolver the chat plane uses, so a model string
// behaves identically on both routes; what differs is only the transport, which is
// exactly what §8.1's per-kind placement exists for. The key id is the
// authenticated caller's, recorded on both rows the call writes.
//
// A refusal before the call — an unresolvable model, a combo, a provider without
// an embeddings block or base URL, no usable account — leaves one request log row
// and no usage row (register G20), the same shape the chat plane gives a request
// refused before the pipeline ran.
func (s *EmbeddingsService) Embed(ctx context.Context, req schema.EmbeddingsRequest, keyID string) (schema.EmbeddingsResponse, dataplane.Outcome, error) {
	call, err := s.resolveCall(ctx, req)
	if err != nil {
		s.recorder.refuse(ctx, call.outcome, keyID, err)
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}
	answer, err := s.perform(ctx, call.request, call.selection, call.outcome, keyID)
	if err != nil {
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}
	return normalizeEmbeddings(answer.Body, req, call.modelID, call.gemini), call.outcome, nil
}

// embeddingModelQuery reports the query parameter a media block declares the model
// under. The reference carries the model in the payload for OpenAI-shaped targets,
// so this is empty unless a kind names it explicitly.
func embeddingModelQuery(_ registry.MediaConfig, _ string) (string, string) {
	return "", ""
}

// embeddingsBody builds the payload: the Gemini shape for a Gemini embedding
// endpoint, and the OpenAI shape for everything else.
func embeddingsBody(req schema.EmbeddingsRequest, upstreamModel string, media registry.MediaConfig) []byte {
	if dataplane.IsGeminiEmbedding(media) {
		input := dataplane.GeminiEmbeddingInput{Single: strings.Join(req.Input.Texts, "\n"), Many: nil}
		if !req.Input.Single() {
			input = dataplane.GeminiEmbeddingInput{Many: req.Input.Texts}
		}
		if req.Dimensions != nil {
			input.Dimensions = *req.Dimensions
		}
		return dataplane.GeminiEmbeddingsBody(upstreamModel, input)
	}

	body := embeddingsUpstreamBody{Model: upstreamModel, Input: req.Input}
	if req.EncodingFormat != "" {
		body.EncodingFormat = req.EncodingFormat
	}
	if req.Dimensions != nil {
		body.Dimensions = req.Dimensions
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return []byte(`{}`)
	}
	return encoded
}

// embeddingsUpstreamBody is the OpenAI embeddings payload, typed so the request
// builder never holds an untyped map (AGENTS.md §1.4). Input marshals itself:
// a single input is a string, a batch an array (schema.EmbeddingInput).
type embeddingsUpstreamBody struct {
	Model          string                `json:"model"`
	Input          schema.EmbeddingInput `json:"input"`
	EncodingFormat string                `json:"encoding_format,omitempty"`
	Dimensions     *int                  `json:"dimensions,omitempty"`
}
