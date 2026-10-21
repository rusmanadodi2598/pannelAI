// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_agnes.go
// @for       The Agnes AI video dialect: the task-id encoding its poll needs, the poll request, and the answer reader.
// @uses      internal/dataplane, internal/schema, encoding/base64, encoding/json, net/url, strings.
// @reason    Agnes retrieves a video by GET /agnesapi?video_id=<id>&model_name=<model>, a path outside the create URL's collection, and keyframe and reference tasks are refused without the model name. The client polls with one opaque task id, so the creation encodes the video id and model into it and the poll decodes them; that keeps every mode pollable without a second round trip.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// agnesVideoAnswer is Agnes's job shape in both phases. A creation names the
// retrieval id in `video_id` beside the task's own `id` and `task_id`; a poll
// repeats only the task id and adds the asset in `url`.
type agnesVideoAnswer struct {
	ID       string          `json:"id"`
	TaskID   string          `json:"task_id"`
	VideoID  string          `json:"video_id"`
	Model    string          `json:"model"`
	Status   string          `json:"status"`
	URL      string          `json:"url"`
	Progress json.RawMessage `json:"progress"`
	Error    json.RawMessage `json:"error"`
}

// readAgnesVideoJob reads Agnes's answer. The creation's task id is encoded
// with the model because the provider's poll needs both; a poll keeps the id
// the request named, which PollVideoJob fills in when the answer repeats none.
func readAgnesVideoJob(body []byte, requireTask bool) (schema.VideoJob, error) {
	var answer agnesVideoAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return schema.VideoJob{}, unreadableVideoAnswer(err)
	}
	job := schema.VideoJob{
		Status:   canonicalVideoStatus(answer.Status, agnesVideoStatuses),
		URL:      answer.URL,
		Progress: videoProgressFrom(answer.Progress),
		Error:    videoErrorMessage(answer.Error),
	}
	if requireTask {
		// video_id is the retrieval id the provider documents; the task ids are
		// the fallback for a response that names only one of them.
		retrievalID := firstNonEmpty(answer.VideoID, answer.TaskID, answer.ID)
		if retrievalID != "" {
			job.TaskID = agnesTaskID(retrievalID, answer.Model)
		}
	}
	return finishVideoJob(job, requireTask)
}

// agnesVideoStatuses is Agnes's job vocabulary, mapped onto the envelope's four
// states.
var agnesVideoStatuses = map[string]string{
	"queued":      schema.VideoJobQueued,
	"in_progress": schema.VideoJobInProgress,
	"processing":  schema.VideoJobInProgress,
	"completed":   schema.VideoJobSucceeded,
	"failed":      schema.VideoJobFailed,
}

// agnesTaskID encodes the retrieval id together with the model the provider
// requires for a poll. Both travel as one opaque task id because the poll route
// carries no model of its own.
func agnesTaskID(videoID, model string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(videoID) + "|" + strings.TrimSpace(model)))
}

// splitAgnesTaskID reverses agnesTaskID. A client polling with the provider's
// own video id is accepted too: the text mode reads it without a model name,
// and the poll omits the parameter rather than guessing one.
func splitAgnesTaskID(taskID string) (string, string) {
	raw := strings.TrimSpace(taskID)
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return raw, ""
	}
	videoID, model, found := strings.Cut(string(decoded), "|")
	if !found || strings.TrimSpace(videoID) == "" {
		return raw, ""
	}
	return strings.TrimSpace(videoID), strings.TrimSpace(model)
}

// buildAgnesVideoPoll builds the retrieval request. The provider serves it from
// /agnesapi on the same host as the create endpoint, not from the collection.
func buildAgnesVideoPoll(call MediaCall, taskID string, _ []byte) (dataplane.MediaRequest, error) {
	videoID, model := splitAgnesTaskID(taskID)
	if videoID == "" {
		return dataplane.MediaRequest{}, dataplane.ValidationError("a video task id is required to poll a job")
	}
	target := strings.TrimSpace(call.BaseURL)
	if target == "" {
		target = call.Target
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return dataplane.MediaRequest{}, dataplane.ValidationError("the video target could not be read")
	}
	parsed.Path = "/agnesapi"
	parsed.RawPath = ""
	query := parsed.Query()
	query.Set("video_id", videoID)
	if model != "" {
		query.Set("model_name", model)
	}
	parsed.RawQuery = query.Encode()
	return dataplane.MediaRequest{Method: "GET", URL: parsed.String()}, nil
}
