// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_plan.go
// @for       The per-provider video dialects: where a creation lands, how a poll is built, and which reader reads the answers.
// @uses      internal/dataplane, internal/schema, strings.
// @reason    A video generation is an async job, and the reference carries one adapter per provider for it (videoProviders/index.js): the xAI shape posts to the collection's `/generations`, openrouter posts to the collection itself, and vertex names the project and model in the path and translates the body both ways. The table lives apart from the use cases so media_video.go stays inside the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// videoPlan is one provider's video dialect: where a creation POST lands
// relative to the block's base URL, how each phase's request is built, and how
// its answers are read.
type videoPlan struct {
	// createPath is appended to the base URL for a creation POST. Empty means
	// the base URL is the collection itself, which is how openrouter and
	// tokenrouter declare theirs. It is ignored when create builds its own
	// request.
	createPath string
	// create builds a provider's creation request when a path append cannot
	// express its dialect: vertex names the project, location, and model in the
	// path and translates the body. Nil keeps the default, a POST to
	// createPath with the model-rewritten body.
	create videoRequestBuilder
	// poll builds a provider's poll request when a path append cannot express
	// its dialect: vertex posts the operation name back to the model path. Nil
	// keeps the default, a GET of the task URL.
	poll videoRequestBuilder
	// read turns one answer into the job it reports, in whichever of the two
	// phases asked. requireTask is true for a creation, which must name the job
	// the client polls with.
	read videoReader
}

// videoRequestBuilder builds one phase's request where a path append cannot
// express the dialect. taskID is the client's id for a poll and empty for a
// creation; raw is the client's own body for a creation and nil for a poll.
type videoRequestBuilder func(call MediaCall, taskID string, raw []byte) (dataplane.MediaRequest, error)

// videoPlans is the per-provider table. A provider the table does not name
// speaks the xAI shape, which is the reference's own default
// (videoProviders/index.js:3-5): the creation posts to `/generations` and the
// poll reads `request_id`, `status`, and `video.url`.
var videoPlans = map[string]videoPlan{
	"agnes":       {poll: buildAgnesVideoPoll, read: readAgnesVideoJob},
	"openrouter":  {read: readOpenRouterVideoJob},
	"tokenrouter": {read: readTokenRouterVideoJob},
	"vertex":      {create: buildVertexVideoCreation, poll: buildVertexVideoPoll, read: readVertexVideoJob},
}

// videoPlanFor answers one provider's dialect, defaulting to the xAI shape.
func videoPlanFor(providerID string) videoPlan {
	if plan, ok := videoPlans[strings.ToLower(strings.TrimSpace(providerID))]; ok {
		return plan
	}
	return videoPlan{createPath: "/generations", read: readXAIVideoJob}
}

// creationRequest builds the creation request for one provider: the default
// rewrites the model member and posts to the plan's path, and a provider whose
// dialect needs more supplies its own builder.
func (p videoPlan) creationRequest(call MediaCall, raw []byte) (dataplane.MediaRequest, error) {
	if p.create != nil {
		return p.create(call, "", raw)
	}
	body, err := schema.RewriteVideoModel(raw, call.UpstreamModel)
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	target, err := dataplane.MediaPath(call.Target, p.createPath)
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	return dataplane.MediaRequest{Method: "POST", URL: target, Body: body}, nil
}

// pollRequest builds the poll request for one provider: the default GETs the
// task URL, and a provider whose poll carries a body supplies its own builder.
func (p videoPlan) pollRequest(call MediaCall, taskID string) (dataplane.MediaRequest, error) {
	if p.poll != nil {
		return p.poll(call, taskID, nil)
	}
	target, err := videoTaskURL(call.Target, taskID)
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	return dataplane.MediaRequest{Method: "GET", URL: target}, nil
}
