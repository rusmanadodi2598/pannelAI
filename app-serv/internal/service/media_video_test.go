// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_test.go
// @for       The async video job's creation: the plan per provider, the model rewrite the upstream body gets, and the refusal of an answer without a job.
// @uses      internal/dataplane, internal/schema, context, strings, testing.
// @reason    SPEC-API-001 §7.10 serves one async video route and the reference carries a per-provider adapter for it (videoProviders/index.js: xAI shape by default, openrouter at the collection root). Each provider's URL, id member, and answer shape is pinned here, because a wrong plan is a billable POST to an address nothing answers and a 200 naming no job must not be served as success (the G17 rule). The poll side lives in media_video_poll_test.go.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// videoRequest is the client body every create case sends.
const videoRequest = `{"model":"%s/grok-imagine-video","prompt":"a paper boat","duration":8}`

// TestMediaCallService_GenerateVideoJob_CreatePlan pins where each provider's
// creation POST lands and what the normalized answer is: the xAI shape posts to
// the collection's `/generations` and reports `request_id`, while openrouter and
// tokenrouter post to the collection itself and report `id` and a `data`-wrapped
// `task_id`.
func TestMediaCallService_GenerateVideoJob_CreatePlan(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		answer     string
		wantURL    string
		wantModel  string
		wantTaskID string
	}{
		{
			name:       "the xAI shape posts to /generations",
			model:      "xai/grok-imagine-video",
			answer:     `{"request_id":"req-1"}`,
			wantURL:    "https://api.x.ai/v1/videos/generations",
			wantModel:  "grok-imagine-video",
			wantTaskID: "req-1",
		},
		{
			name:       "openrouter posts to the collection root",
			model:      "openrouter/google/veo-3.1",
			answer:     `{"id":"job-1","status":"pending"}`,
			wantURL:    "https://openrouter.ai/api/v1/videos",
			wantModel:  "google/veo-3.1",
			wantTaskID: "job-1",
		},
		{
			name:       "agnes posts to its collection and encodes the retrieval id",
			model:      "agnes/agnes-video-2.5-flash",
			answer:     `{"id":"task_1","task_id":"task_1","video_id":"video_1","model":"agnes-video-2.5-flash","status":"queued"}`,
			wantURL:    "https://apihub.agnes-ai.com/v1/videos",
			wantModel:  "agnes-video-2.5-flash",
			wantTaskID: "dmlkZW9fMXxhZ25lcy12aWRlby0yLjUtZmxhc2g",
		},
		{
			name:       "tokenrouter posts to its declared collection and reads the envelope",
			model:      "tokenrouter/kling-v1",
			answer:     `{"code":0,"message":"ok","data":{"task_id":"tr-1","status":"QUEUED"}}`,
			wantURL:    "https://api.tokenrouter.com/v1/video/generations",
			wantModel:  "kling-v1",
			wantTaskID: "tr-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(tc.answer)}
			providerID, _, found := strings.Cut(tc.model, "/")
			if !found {
				t.Fatalf("the fixture model %q carries no provider", tc.model)
			}

			job, outcome, err := svc.GenerateVideoJob(context.Background(), schema.VideoRequest{
				Model: tc.model, Prompt: "a paper boat",
			}, []byte(strings.Replace(videoRequest, "%s", providerID, 1)), "")
			if err != nil {
				t.Fatalf("GenerateVideoJob() error = %v", err)
			}
			if job.TaskID != tc.wantTaskID {
				t.Fatalf("task id = %q, want %q", job.TaskID, tc.wantTaskID)
			}
			if job.Status != schema.VideoJobQueued {
				t.Fatalf("status = %q, want queued: a creation is a queued job", job.Status)
			}
			if len(caller.requests) != 1 {
				t.Fatalf("outbound calls = %d, want 1", len(caller.requests))
			}
			if got := caller.requests[0].URL; got != tc.wantURL {
				t.Fatalf("create URL = %q, want %q", got, tc.wantURL)
			}
			if got := caller.requests[0].Method; got != "POST" {
				t.Fatalf("create method = %q, want POST", got)
			}
			// The upstream must read the id it knows, with the client's own
			// provider-specific members still on the body.
			body := string(caller.requests[0].Body)
			if !strings.Contains(body, `"model":"`+tc.wantModel+`"`) {
				t.Fatalf("upstream body = %s, want the rewritten model %q", body, tc.wantModel)
			}
			if !strings.Contains(body, `"duration":8`) {
				t.Fatalf("upstream body = %s, want the client's own duration member", body)
			}
			// The endpoint the job was created on is what the client pins its
			// polls to, so the outcome must name it.
			if outcome.EndpointID != "ep-"+providerID {
				t.Fatalf("outcome endpoint = %q, want the created endpoint", outcome.EndpointID)
			}
			if len(router.pinned) != 0 {
				t.Fatalf("pinned = %v, want no pin on a create", router.pinned)
			}
		})
	}
}

// TestMediaCallService_GenerateVideoJob_RefusesAnAnswerWithoutAJob pins the G17
// rule for this route: a 200 that names no job is a failure the client sees,
// not a served call, because there is nothing to poll.
func TestMediaCallService_GenerateVideoJob_RefusesAnAnswerWithoutAJob(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"status":"pending"}`)}

	_, _, err := svc.GenerateVideoJob(context.Background(), schema.VideoRequest{
		Model: "xai/grok-imagine-video", Prompt: "a paper boat",
	}, []byte(`{"model":"xai/grok-imagine-video","prompt":"a paper boat"}`), "")
	if err == nil || !strings.Contains(err.Error(), "video job") {
		t.Fatalf("error = %v, want the unreadable-answer refusal", err)
	}
	if len(router.failures) == 0 {
		t.Fatal("the endpoint's health recorded nothing, want the 200 classified as a failure")
	}
}
