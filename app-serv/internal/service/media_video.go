// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video.go
// @for       The async video generation use cases of SPEC-API-001 §7.10: creating a job and polling one.
// @uses      internal/dataplane, internal/domain, internal/schema, context, encoding/json, net/url, strings.
// @reason    A video generation is an async job rather than a generated asset, so the two routes are a creation and a poll rather than one call. The reference carries a per-provider adapter for the same reason (videoProviders/index.js): the xAI shape posts to the collection's `/generations`, openrouter posts to the collection itself, and each names its job differently. The per-provider differences are one table here, and the answer reading lives in media_video_read.go (AGENTS.md §1.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// videoPlan is one provider's video dialect: where a creation POST lands
// relative to the block's base URL, and how its answers are read.
type videoPlan struct {
	// createPath is appended to the base URL for a creation POST. Empty means
	// the base URL is the collection itself, which is how openrouter and
	// tokenrouter declare theirs.
	createPath string
	// read turns one answer into the job it reports, in whichever of the two
	// phases asked. requireTask is true for a creation, which must name the job
	// the client polls with.
	read videoReader
}

// videoPlans is the per-provider table. A provider the table does not name
// speaks the xAI shape, which is the reference's own default
// (videoProviders/index.js:3-5): the creation posts to `/generations` and the
// poll reads `request_id`, `status`, and `video.url`.
var videoPlans = map[string]videoPlan{
	"openrouter":  {read: readOpenRouterVideoJob},
	"tokenrouter": {read: readTokenRouterVideoJob},
}

// videoPlanFor answers one provider's dialect, defaulting to the xAI shape.
func videoPlanFor(providerID string) videoPlan {
	if plan, ok := videoPlans[strings.ToLower(strings.TrimSpace(providerID))]; ok {
		return plan
	}
	return videoPlan{createPath: "/generations", read: readXAIVideoJob}
}

// GenerateVideoJob creates one async video job and normalizes the job it
// answers with.
//
// The client's own body travels to the upstream with one member rewritten: the
// model string it addressed is replaced by the id the upstream expects, and
// every provider-specific member it sent (a duration, a resolution) is kept,
// which is why the rewrite is the schema's and not a typed body here.
func (s *MediaCallService) GenerateVideoJob(ctx context.Context, req schema.VideoRequest, raw []byte, keyID string) (schema.VideoJob, dataplane.Outcome, error) {
	call, err := s.prepareForCall(ctx, req.Model, domain.MediaKindVideo, nil, keyID)
	if err != nil {
		return schema.VideoJob{}, dataplane.Outcome{}, err
	}
	body, err := schema.RewriteVideoModel(raw, call.UpstreamModel)
	if err != nil {
		return schema.VideoJob{}, call.Outcome(), err
	}
	plan := videoPlanFor(call.ProviderID)
	target, err := dataplane.MediaPath(call.Target, plan.createPath)
	if err != nil {
		return schema.VideoJob{}, call.Outcome(), err
	}
	answer, err := s.Perform(ctx, call, dataplane.MediaRequest{Method: "POST", URL: target, Body: body},
		keyID, plan.reader(true))
	if err != nil {
		return schema.VideoJob{}, call.Outcome(), err
	}
	job, err := videoJobFromAnswer(answer.Body)
	return job, call.Outcome(), err
}

// PollVideoJob reads the state of one job the gateway created.
//
// A job lives on the account that created it, so the poll prefers that account
// when the client names it, which is why the creation answers the endpoint it
// used (the reference's `x-9router-connection-id`). The call records its usage
// and log rows like every served media call, but it writes no endpoint health:
// a poll is a read of a job that already exists, and a stale task id answering
// 404 would otherwise park a healthy key for the failure window the reference's
// own 404 rule gives it (docs/PORT/011-PORT-VIDEO-ASYNC-JOB.md D5).
func (s *MediaCallService) PollVideoJob(ctx context.Context, providerID, endpointID, taskID, keyID string) (schema.VideoJob, dataplane.Outcome, error) {
	call, err := s.preparePinned(ctx, providerID, endpointID, keyID)
	if err != nil {
		return schema.VideoJob{}, dataplane.Outcome{}, err
	}
	call.SkipHealth = true
	target, err := videoTaskURL(call.Target, taskID)
	if err != nil {
		return schema.VideoJob{}, call.Outcome(), err
	}
	answer, err := s.Perform(ctx, call, dataplane.MediaRequest{Method: "GET", URL: target},
		keyID, videoPlanFor(call.ProviderID).reader(false))
	if err != nil {
		return schema.VideoJob{}, call.Outcome(), err
	}
	job, err := videoJobFromAnswer(answer.Body)
	if err != nil {
		return schema.VideoJob{}, call.Outcome(), err
	}
	// A provider whose poll reports the state alone (xAI) does not repeat the
	// id, so the envelope carries the one the request named.
	if job.TaskID == "" {
		job.TaskID = strings.TrimSpace(taskID)
	}
	return job, call.Outcome(), nil
}

// preparePinned resolves one provider's video block around a named endpoint.
// It resolves without selecting, so the rotation cursor is not advanced by a
// call that pins its own account, and binds the credential of the account the
// pin names (or the selected one when the client named none).
func (s *MediaCallService) preparePinned(ctx context.Context, providerID, endpointID, keyID string) (MediaCall, error) {
	call, err := s.resolveTarget(ctx, strings.TrimSpace(providerID)+"/", domain.MediaKindVideo, false)
	if err != nil {
		s.recorder.refuse(ctx, mediaRefusalOutcome(providerID, ""), keyID, err)
		return MediaCall{}, err
	}
	selection, err := s.selectForPoll(ctx, call.ProviderID, endpointID)
	if err != nil {
		s.recorder.refuse(ctx, call.Outcome(), keyID, err)
		return MediaCall{}, err
	}
	bound, err := s.bindCredential(call, selection, nil)
	if err != nil {
		s.recorder.refuse(ctx, call.Outcome(), keyID, err)
		return MediaCall{}, err
	}
	return bound, nil
}

// selectForPoll pins the account a poll names and falls back to the provider's
// own selection when it names none. That is the reference's preference, not a
// hard pin: its poll reads the connection id off the request and lets the
// credential walk answer when the header is absent (videoGeneration.js:25-33,
// :203-209).
func (s *MediaCallService) selectForPoll(ctx context.Context, providerID, endpointID string) (dataplane.Selection, error) {
	if strings.TrimSpace(endpointID) == "" {
		return s.router.Select(ctx, providerID)
	}
	return s.router.SelectEndpoint(ctx, providerID, endpointID)
}

// videoTaskURL appends one task id to the block's target as a single escaped
// path segment, which is the reference's `encodeURIComponent(requestId)`
// (videoCore.js:34-36): an id carrying a slash must not address a deeper path.
func videoTaskURL(target, taskID string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return "", dataplane.ValidationError("the video target could not be read")
	}
	id := strings.TrimSpace(taskID)
	if id == "" {
		return "", dataplane.ValidationError("a video task id is required to poll a job")
	}
	escapedBase := strings.TrimSuffix(parsed.EscapedPath(), "/")
	base := strings.TrimSuffix(parsed.Path, "/")
	parsed.Path = base + "/" + id
	// RawPath carries the escaping String() would otherwise apply twice: the
	// id is already escaped, and re-encoding it would turn %2F into %252F.
	parsed.RawPath = escapedBase + "/" + url.PathEscape(id)
	return parsed.String(), nil
}

// videoJobFromAnswer reads the canonical envelope a reader produced. A reader
// that answered something else is a failure rather than an empty job: the
// client's next move is to poll, and polling an id nothing created would look
// like the upstream's fault.
func videoJobFromAnswer(body []byte) (schema.VideoJob, error) {
	var job schema.VideoJob
	if err := json.Unmarshal(body, &job); err != nil {
		return schema.VideoJob{}, dataplane.InternalError("the video answer could not be read", err)
	}
	return job, nil
}
