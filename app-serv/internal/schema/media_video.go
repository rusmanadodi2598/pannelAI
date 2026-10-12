// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_video.go
// @for       The async video-job contract of SPEC-API-001 §7.10: the create request, the normalized job envelope, and the model rewrite the upstream body gets.
// @uses      encoding/json, internal/domain.
// @reason    A video generation is an async job rather than a generated asset: the create answers a job id the client polls, so the route's answer is its own envelope and not the image one. The request is read for two fields only (the model the gateway routes by and the prompt), and every other member travels to the upstream as the client wrote it, which is why the rewrite rewrites one member instead of re-encoding a typed body that would drop the provider's own knobs.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-12
package schema

import (
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// VideoJobStatus is the canonical job state the gateway reports, whichever
// provider answered. The provider's own vocabulary is mapped onto these four by
// the video service; a state no mapping knows is passed through lower-cased
// rather than guessed at, so a client polling for a terminal state is never
// told a job finished when it did not.
const (
	VideoJobQueued     = "queued"
	VideoJobInProgress = "in_progress"
	VideoJobSucceeded  = "succeeded"
	VideoJobFailed     = "failed"
)

// VideoRequest is the body of POST /api/v1/videos/generations. Model is the
// string the gateway resolves the provider from, and prompt is what the
// upstream generates; every other member is provider-specific and is forwarded
// untouched.
type VideoRequest struct {
	Model  string `json:"model" validate:"required,min=1,max=200"`
	Prompt string `json:"prompt" validate:"required,min=1,max=32000"`
}

// VideoJob is the normalized envelope both video routes answer with: the job
// the upstream created, in one shape whichever provider produced it. TaskID is
// the id the poll route takes, Status one of the canonical states, URL the
// asset once the job succeeded, and Error the upstream's own message once it
// failed.
type VideoJob struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
	// Progress is the upstream's own percentage when it reports one. A pointer
	// so a job that reports zero stays distinguishable from one that reports
	// nothing.
	Progress *int   `json:"progress,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DecodeVideoRequest decodes a video body into its typed contract.
func DecodeVideoRequest(raw []byte) (VideoRequest, error) {
	var req VideoRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return VideoRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}

// RewriteVideoModel returns the client's own body with its model member
// replaced by the id the upstream expects.
//
// A body that already names that id is answered byte-identical, which is the
// reference's own rule (videoGeneration.js:120-124): the gateway only rewrites
// what it must, so the upstream reads the body the client wrote rather than the
// gateway's re-encoding of it.
func RewriteVideoModel(raw []byte, upstreamModel string) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	if body == nil {
		return nil, domain.NewValidationError("invalid request body: the body must be a JSON object")
	}
	var named string
	if value, ok := body["model"]; ok {
		if err := json.Unmarshal(value, &named); err != nil {
			return nil, domain.NewValidationError("invalid request body: field model must be a string")
		}
	}
	if named == upstreamModel {
		return raw, nil
	}
	encoded, err := json.Marshal(upstreamModel)
	if err != nil {
		return nil, domain.NewInternalError("the model could not be written into the video request")
	}
	body["model"] = encoded
	rewritten, err := json.Marshal(body)
	if err != nil {
		return nil, domain.NewInternalError("the video request could not be built")
	}
	return rewritten, nil
}
