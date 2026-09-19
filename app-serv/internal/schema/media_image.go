// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_image.go
// @for       The image and video generation contracts of SPEC-API-001 §7.10.
// @uses      encoding/json, internal/domain.
// @reason    Both routes send a prompt to a provider's generation endpoint and
//
//	answer with the same OpenAI envelope the reference normalizes to,
//	so the request bodies differ only in the knobs the reference
//	forwards for images. §7.10's video route declares no provider yet,
//	and the type is still written because the route is registered.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
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

// VideoRequest is the body of POST /api/v1/videos/generations. The reference's
// dashboard sends a model and a prompt and reads `data[].url` back.
type VideoRequest struct {
	Model  string `json:"model" validate:"required,min=1,max=200"`
	Prompt string `json:"prompt" validate:"required,min=1,max=32000"`
	Size   string `json:"size,omitempty" validate:"omitempty,max=32"`
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

// VideoBody is the payload a video generation endpoint expects.
type VideoBody struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size,omitempty"`
}

// MediaGenerationResponse is the OpenAI envelope both generation routes answer
// with, whichever upstream produced it.
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

// DecodeVideoRequest decodes a video body into its typed contract.
func DecodeVideoRequest(raw []byte) (VideoRequest, error) {
	var req VideoRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return VideoRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}
