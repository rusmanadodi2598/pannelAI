// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_image.go
// @for       The image generation contract of SPEC-API-001 §7.10.
// @uses      encoding/json, internal/domain.
// @reason    The image route sends a prompt to a provider's generation endpoint and answers with the OpenAI envelope the reference normalizes to. The video route's own contract lives in media_video.go, because a video generation is an async job with a different request and a different answer.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-19
package schema

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ImageRequest is the body of POST /api/v1/images/generations.
type ImageRequest struct {
	Model          string `json:"model" validate:"required,min=1,max=200"`
	Prompt         string `json:"prompt" validate:"required,min=1,max=32000"`
	N              *int   `json:"n,omitempty" validate:"omitempty,gte=1,lte=10"`
	Size           string `json:"size,omitempty" validate:"omitempty,max=32"`
	Quality        string `json:"quality,omitempty" validate:"omitempty,max=32"`
	Style          string `json:"style,omitempty" validate:"omitempty,oneof=vivid natural"`
	ResponseFormat string `json:"response_format,omitempty" validate:"omitempty,oneof=url b64_json"`
}

// ImageBody is the payload an OpenAI-compatible generation endpoint expects.
type ImageBody struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              *int   `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	Style          string `json:"style,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

// MediaGenerationResponse is the OpenAI envelope the image route answers with,
// whichever upstream produced it.
type MediaGenerationResponse struct {
	Created int64                   `json:"created"`
	Data    []MediaGenerationObject `json:"data"`
}

// MediaGenerationObject is one generated asset: a URL, or the bytes in base64
// when the caller asked for that form.
type MediaGenerationObject struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// DecodeImageRequest decodes an image body into its typed contract.
func DecodeImageRequest(raw []byte) (ImageRequest, error) {
	var req ImageRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return ImageRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
