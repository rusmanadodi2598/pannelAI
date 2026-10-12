// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_image.go
// @for       The image generation use case of SPEC-API-001 §7.10.
// @uses      internal/dataplane, internal/domain, internal/schema, context, encoding/json.
// @reason    The route sends a prompt and normalizes the answer into the OpenAI envelope, so the normalization lives here once: the reference normalizes every adapter's answer into `{created, data:[{url|b64_json}]}`, and a client that reads `data[0].url` must keep working whichever provider answered. The video route's own use cases live in media_video.go, because a video generation is an async job with its own envelope.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// GenerateImage performs one image generation call and normalizes the answer.
func (s *MediaCallService) GenerateImage(ctx context.Context, req schema.ImageRequest, keyID string) (schema.MediaGenerationResponse, dataplane.Outcome, error) {
	call, err := s.prepareForCall(ctx, req.Model, domain.MediaKindImage, nil, keyID)
	if err != nil {
		return schema.MediaGenerationResponse{}, dataplane.Outcome{}, err
	}
	body, err := json.Marshal(schema.ImageBody{
		Model: call.UpstreamModel, Prompt: req.Prompt, N: req.N, Size: req.Size,
		Quality: req.Quality, Style: req.Style, ResponseFormat: req.ResponseFormat,
	})
	if err != nil {
		return schema.MediaGenerationResponse{}, call.Outcome(),
			dataplane.InternalError("the image request could not be built", err)
	}
	answer, err := s.Perform(ctx, call, dataplane.MediaRequest{Method: "POST", Body: body}, keyID, nil)
	if err != nil {
		return schema.MediaGenerationResponse{}, call.Outcome(), err
	}
	normalized, err := normalizeGeneration(answer.Body)
	return normalized, call.Outcome(), err
}

// generationEnvelope is the upstream answer the image route normalizes: the
// OpenAI image envelope the reference's adapters produce.
type generationEnvelope struct {
	Created int64                          `json:"created"`
	Data    []schema.MediaGenerationObject `json:"data"`
}

// normalizeGeneration reads an upstream answer into the OpenAI envelope.
//
// An answer that is not JSON is refused rather than forwarded: a 200 carrying a
// proxy's error page would otherwise reach the client as a generation result it
// cannot use, and the failure would look like the provider's.
func normalizeGeneration(body []byte) (schema.MediaGenerationResponse, error) {
	var upstream generationEnvelope
	if err := json.Unmarshal(body, &upstream); err != nil {
		return schema.MediaGenerationResponse{}, dataplane.InternalError(
			"the generation answer could not be read", err)
	}
	if upstream.Data == nil {
		upstream.Data = []schema.MediaGenerationObject{}
	}
	return schema.MediaGenerationResponse{Created: upstream.Created, Data: upstream.Data}, nil
}
