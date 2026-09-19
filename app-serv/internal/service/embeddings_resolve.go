// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_resolve.go
// @for       The resolution phase of one embeddings call: model to provider,
//
//	provider to media block and base URL, provider to account, and the
//	identity a refusal can still name.
//
// @uses      internal/dataplane, internal/domain, internal/registry,
//
//	internal/schema, context, strings.
//
// @reason    Register G20: a refusal before the call must leave a log row with
//
//	the identity resolved so far, and the cleanest way to know how far
//	resolution got is to build the identity before the first fallible
//	step and return it with the error — the same rule G17's relayOnce
//	settled for the chat plane. Splitting the phase here also keeps
//	embeddings.go inside the §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// embeddingsCall is one resolved embeddings call: the request to send, the
// account it uses, and the identity its accounting rows carry. The outcome is
// built before the first fallible step, so a refusal returns the call with
// whatever identity resolution had already established (register G20).
type embeddingsCall struct {
	outcome   dataplane.Outcome
	request   dataplane.MediaRequest
	selection dataplane.Selection
	modelID   string
	gemini    bool
}

// resolveCall runs everything that happens before the upstream call: resolve
// the model, refuse a combo, read the provider's embeddings block and base URL,
// select an account, and build the target. Every failure returns the identity
// resolved so far alongside the error, so the caller records the refusal against
// the provider and model it was aimed at.
func (s *EmbeddingsService) resolveCall(ctx context.Context, req schema.EmbeddingsRequest) (embeddingsCall, error) {
	resolution, err := s.resolver.Resolve(ctx, req.Model)
	if err != nil {
		return embeddingsCall{}, err
	}
	call := embeddingsCall{
		modelID: resolution.ModelID,
		outcome: dataplane.Outcome{
			Format:     schema.FormatOpenAI,
			ProviderID: resolution.Provider.ID,
			Model:      resolution.ModelID,
		},
	}
	if resolution.IsCombo() {
		// A combo is an ordered list for chat failover. Honouring its first member
		// would embed with a model the client did not name, so it is refused rather
		// than silently dereferenced.
		return call, dataplane.ValidationError(
			"model " + req.Model + " is a combo; embeddings requires a single model")
	}

	media, baseURL, err := s.mediaConfig(ctx, resolution.Provider)
	if err != nil {
		return call, err
	}

	selection, err := s.router.Select(ctx, resolution.Provider.ID)
	if err != nil {
		return call, err
	}
	call.selection = selection
	call.outcome.EndpointID = selection.Endpoint.ID()

	gemini := dataplane.IsGeminiEmbedding(media)
	query := make(map[string]string, 1)
	if !gemini {
		if name, value := embeddingModelQuery(media, resolution.UpstreamID); name != "" {
			query[name] = value
		}
	}
	target, headers, err := dataplane.MediaTarget(media, baseURL, selection.Credential, query)
	if err != nil {
		return call, err
	}
	if gemini {
		target = dataplane.GeminiEmbeddingsURL(baseURL, resolution.UpstreamID, !req.Input.Single())
	}

	call.gemini = gemini
	call.request = dataplane.MediaRequest{
		Method: "POST", URL: target, Headers: headers, Body: embeddingsBody(req, resolution.UpstreamID, media),
		TimeoutMS: media.TimeoutMS,
	}
	return call, nil
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
