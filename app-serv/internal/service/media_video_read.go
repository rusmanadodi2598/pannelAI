// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_read.go
// @for       Reading one provider's video answer into the normalized job envelope, and the per-provider status vocabulary that mapping needs.
// @uses      internal/dataplane, internal/domain, internal/schema, encoding/json, strings.
// @reason    A provider can report a failure inside a 200, and a creation that names no job is a call the client cannot follow up, so reading the answer is part of classifying it: the reader refuses such an answer and the call is recorded the way the client experienced it (the G17 rule, media_perform.go). Each provider's own id member and status words are ported here so the envelope means the same thing whichever upstream answered.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// videoReader reads one provider's answer into the envelope. requireTask is
// true for a creation answer, which must name the job, and false for a poll
// answer, whose id the request already carries: xAI's poll reports the state
// alone (xai-video-core.test.js:116-135).
type videoReader func(body []byte, requireTask bool) (schema.VideoJob, error)

// reader adapts one provider's reader to the seam media_perform.go reads an
// answer through.
func (p videoPlan) reader(requireTask bool) mediaAnswerReader {
	return func(_ int, body []byte) ([]byte, error) {
		job, err := p.read(body, requireTask)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(job)
		if err != nil {
			return nil, dataplane.InternalError("the video job could not be built", err)
		}
		return encoded, nil
	}
}

// xaiVideoAnswer is the xAI shape, which is also the default: a creation
// answers `request_id`, and a poll answers `status`, an optional `progress`,
// `video.url` once the job succeeded, and `error` once it failed.
type xaiVideoAnswer struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Progress  *int   `json:"progress"`
	Video     struct {
		URL string `json:"url"`
	} `json:"video"`
	Error json.RawMessage `json:"error"`
}

// readXAIVideoJob reads the xAI shape.
func readXAIVideoJob(body []byte, requireTask bool) (schema.VideoJob, error) {
	var answer xaiVideoAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return schema.VideoJob{}, unreadableVideoAnswer(err)
	}
	return finishVideoJob(schema.VideoJob{
		TaskID:   answer.RequestID,
		Status:   canonicalVideoStatus(answer.Status, xaiVideoStatuses),
		URL:      answer.Video.URL,
		Progress: answer.Progress,
		Error:    videoErrorMessage(answer.Error),
	}, requireTask)
}

// openRouterVideoAnswer is OpenRouter's job shape: `id` for the job,
// `status`, and the asset in `unsigned_urls` once it completed
// (videoProviders/openrouter.js:1-4, video-providers.test.js:74-88).
type openRouterVideoAnswer struct {
	ID           string          `json:"id"`
	Status       string          `json:"status"`
	UnsignedURLs []string        `json:"unsigned_urls"`
	Error        json.RawMessage `json:"error"`
}

// readOpenRouterVideoJob reads OpenRouter's job shape.
func readOpenRouterVideoJob(body []byte, requireTask bool) (schema.VideoJob, error) {
	var answer openRouterVideoAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return schema.VideoJob{}, unreadableVideoAnswer(err)
	}
	job := schema.VideoJob{
		TaskID: answer.ID,
		Status: canonicalVideoStatus(answer.Status, openRouterVideoStatuses),
		Error:  videoErrorMessage(answer.Error),
	}
	if len(answer.UnsignedURLs) > 0 {
		job.URL = answer.UnsignedURLs[0]
	}
	return finishVideoJob(job, requireTask)
}

// tokenRouterVideoAnswer is TokenRouter's envelope: the job's own fields under
// `data`, beside the gateway-level `code` and `message`. A creation names the
// task at the top level or under `data`, so both are read.
type tokenRouterVideoAnswer struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	ResultURL string `json:"result_url"`
	Progress  *int   `json:"progress"`
	Message   string `json:"message"`
	Data      *struct {
		TaskID    string `json:"task_id"`
		Status    string `json:"status"`
		ResultURL string `json:"result_url"`
		Progress  *int   `json:"progress"`
	} `json:"data"`
}

// readTokenRouterVideoJob reads TokenRouter's envelope.
func readTokenRouterVideoJob(body []byte, requireTask bool) (schema.VideoJob, error) {
	var answer tokenRouterVideoAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return schema.VideoJob{}, unreadableVideoAnswer(err)
	}
	job := schema.VideoJob{
		TaskID:   answer.TaskID,
		Status:   answer.Status,
		URL:      answer.ResultURL,
		Progress: answer.Progress,
	}
	if answer.Data != nil {
		if job.TaskID == "" {
			job.TaskID = answer.Data.TaskID
		}
		if job.Status == "" {
			job.Status = answer.Data.Status
		}
		if job.URL == "" {
			job.URL = answer.Data.ResultURL
		}
		if job.Progress == nil {
			job.Progress = answer.Data.Progress
		}
	}
	job.Status = canonicalVideoStatus(job.Status, tokenRouterVideoStatuses)
	// The envelope's own message is the failure's text: it reads "ok" on a
	// served job and names what went wrong on a failed one.
	if job.Status == schema.VideoJobFailed {
		job.Error = strings.TrimSpace(answer.Message)
	}
	return finishVideoJob(job, requireTask)
}

// xaiVideoStatuses, openRouterVideoStatuses, and tokenRouterVideoStatuses are
// each provider's own job vocabulary, mapped onto the envelope's four states.
// tokenrouter spells the states in capitals and distinguishes a submitted job
// from a running one, which the envelope folds together: a client polling for a
// terminal state reads both as "not yet".
var (
	xaiVideoStatuses = map[string]string{
		"pending":    schema.VideoJobQueued,
		"processing": schema.VideoJobInProgress,
		"done":       schema.VideoJobSucceeded,
		"failed":     schema.VideoJobFailed,
	}
	openRouterVideoStatuses = map[string]string{
		"pending":   schema.VideoJobQueued,
		"completed": schema.VideoJobSucceeded,
		"failed":    schema.VideoJobFailed,
	}
	tokenRouterVideoStatuses = map[string]string{
		"queued":      schema.VideoJobQueued,
		"submitted":   schema.VideoJobQueued,
		"in_progress": schema.VideoJobInProgress,
		"success":     schema.VideoJobSucceeded,
		"failure":     schema.VideoJobFailed,
	}
)

// canonicalVideoStatus maps one provider's state word onto the envelope's,
// lower-casing it first because tokenrouter spells its states in capitals. A
// state no mapping knows is passed through lower-cased rather than guessed at,
// so a client polling for a terminal state is never told a job finished when it
// did not.
func canonicalVideoStatus(upstream string, known map[string]string) string {
	status := strings.ToLower(strings.TrimSpace(upstream))
	if mapped, ok := known[status]; ok {
		return mapped
	}
	return status
}

// finishVideoJob applies the two rules the phases differ in: a creation must
// name the job (nothing else gives the client something to poll), and a
// creation that reports no state yet is a queued job.
func finishVideoJob(job schema.VideoJob, requireTask bool) (schema.VideoJob, error) {
	job.TaskID = strings.TrimSpace(job.TaskID)
	if requireTask && job.TaskID == "" {
		return schema.VideoJob{}, dataplane.UpstreamRejected(200,
			"the video upstream accepted the job but named no video job to poll")
	}
	if requireTask && job.Status == "" {
		job.Status = schema.VideoJobQueued
	}
	if !requireTask && job.Status == "" {
		return schema.VideoJob{}, dataplane.UpstreamRejected(200,
			"the video upstream answered no job state")
	}
	return job, nil
}

// unreadableVideoAnswer reports an answer that is not the JSON job the provider
// documents. A 200 carrying a proxy's error page would otherwise reach the
// client as a job it cannot use, and the failure would look like the
// provider's.
func unreadableVideoAnswer(cause error) error {
	return dataplane.InternalError("the video answer could not be read", cause)
}

// videoErrorMessage reads an upstream's error member, which some providers
// write as a message string and others as an object carrying one.
func videoErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ""
	}
	return strings.TrimSpace(envelope.Message)
}
