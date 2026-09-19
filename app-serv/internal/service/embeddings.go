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
	engine    *dataplane.Engine
	caller    dataplane.MediaCaller
	overrides MediaOverrideReader
	recorder  dataPlaneRecorder
}

// EmbeddingsServiceDeps holds the collaborators the service needs.
type EmbeddingsServiceDeps struct {
	Engine *dataplane.Engine
	Caller dataplane.MediaCaller
	// Overrides reads the stored per-provider base URL §7.10's save writes.
	// It is optional so a deployment that has not configured one still
	// resolves the registry's own base URL.
	Overrides MediaOverrideReader
	// Usage and Logs write the §7.12/§7.13 accounting pair for every call that
	// reaches an upstream; RequestID gives both rows one identifier. All three
	// are optional so a deployment that wires none still serves.
	Usage     UsageRecorder
	Logs      RequestLogRecorder
	RequestID RequestIDReader
}

// NewEmbeddingsService validates deps and returns a ready service.
func NewEmbeddingsService(deps EmbeddingsServiceDeps) (*EmbeddingsService, error) {
	if deps.Engine == nil {
		return nil, domain.NewValidationError("data plane engine is required")
	}
	if deps.Caller == nil {
		return nil, domain.NewValidationError("media caller is required")
	}
	return &EmbeddingsService{
		engine: deps.Engine, caller: deps.Caller, overrides: deps.Overrides,
		recorder: newDataPlaneRecorder(deps.Usage, deps.Logs, deps.RequestID),
	}, nil
}

// Embed resolves the model, selects the account, performs the call, and returns the
// answer in the OpenAI embeddings shape.
//
// Resolution runs through the same resolver the chat plane uses, so a model string
// behaves identically on both routes; what differs is only the transport, which is
// exactly what §8.1's per-kind placement exists for. The key id is the
// authenticated caller's, recorded on both rows the call writes.
func (s *EmbeddingsService) Embed(ctx context.Context, req schema.EmbeddingsRequest, keyID string) (schema.EmbeddingsResponse, dataplane.Outcome, error) {
	resolution, err := s.engine.Resolver().Resolve(ctx, req.Model)
	if err != nil {
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}
	if resolution.IsCombo() {
		// A combo is an ordered list for chat failover. Honouring its first member
		// would embed with a model the client did not name, so it is refused rather
		// than silently dereferenced.
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, dataplane.ValidationError(
			"model " + req.Model + " is a combo; embeddings requires a single model")
	}

	media, baseURL, err := s.mediaConfig(ctx, resolution.Provider)
	if err != nil {
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}

	selection, err := s.engine.Selector().Select(ctx, resolution.Provider.ID)
	if err != nil {
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}

	gemini := dataplane.IsGeminiEmbedding(media)
	query := make(map[string]string, 1)
	if !gemini {
		if name, value := embeddingModelQuery(media, resolution.UpstreamID); name != "" {
			query[name] = value
		}
	}
	target, headers, err := dataplane.MediaTarget(media, baseURL, selection.Credential, query)
	if err != nil {
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}
	if gemini {
		target = dataplane.GeminiEmbeddingsURL(baseURL, resolution.UpstreamID, !req.Input.Single())
	}

	outcome := dataplane.Outcome{
		Format:     schema.FormatOpenAI,
		ProviderID: resolution.Provider.ID,
		EndpointID: selection.Endpoint.ID(),
		Model:      resolution.ModelID,
	}
	answer, err := s.perform(ctx, dataplane.MediaRequest{
		Method: "POST", URL: target, Headers: headers, Body: embeddingsBody(req, resolution.UpstreamID, media),
		TimeoutMS: media.TimeoutMS,
	}, selection, outcome, keyID)
	if err != nil {
		return schema.EmbeddingsResponse{}, dataplane.Outcome{}, err
	}
	return normalizeEmbeddings(answer.Body, req, resolution.ModelID, gemini), outcome, nil
}

// mediaConfig reads the provider's per-kind block and the effective base URL:
// the operator's stored override wins over the registry's own value, and a
// provider with neither is refused because §7.10 forbids a silent cloud
// fallback. The stored value is read per call rather than cached, so a save in
// the panel takes effect on the next request.
//
// A custom OpenAI-compatible node declares no media block at all, so its
// embeddings block is synthesized from the node's base URL before the refusal
// is reached (see nodeEmbeddingMedia).
func (s *EmbeddingsService) mediaConfig(ctx context.Context, entry registry.Provider) (registry.MediaConfig, string, error) {
	media, ok := entry.Media.For(registry.MediaEmbedding)
	if !ok {
		media, ok = nodeEmbeddingMedia(entry)
	}
	if !ok {
		return registry.MediaConfig{}, "", dataplane.ProviderNotRoutable(
			"provider " + entry.ID + " does not offer embeddings")
	}
	baseURL := strings.TrimSpace(media.BaseURL)
	if s.overrides != nil {
		stored, err := s.overrides.MediaBaseURL(ctx, entry.ID, domain.MediaKindEmbedding)
		if err != nil {
			return registry.MediaConfig{}, "", err
		}
		if stored = strings.TrimSpace(stored); stored != "" {
			baseURL = stored
		}
	}
	if baseURL == "" {
		// §7.10 forbids a silent cloud fallback: a provider with no base_url is a
		// configuration mistake, not a default to guess at.
		return registry.MediaConfig{}, "", dataplane.ValidationError(
			"provider " + entry.ID + " has no embeddings base_url configured")
	}
	return media, baseURL, nil
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
