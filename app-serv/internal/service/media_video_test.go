// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_test.go
// @for       The async video job: the create plan per provider, the model rewrite the upstream body gets, the normalized job envelope, and the poll's pinned account.
// @uses      internal/dataplane, internal/schema, context, strings, testing.
// @reason    SPEC-API-001 §7.10 serves one async video route and the reference carries a per-provider adapter for it (videoProviders/index.js: xAI shape by default, openrouter at the collection root). Each provider's URL, id member, and status vocabulary is pinned here, because a wrong plan is a billable POST to an address nothing answers, and a wrong reader reports a failed job as a served call (the G17 rule).
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

// TestMediaCallService_PollVideoJob pins the poll: the pinned endpoint answers,
// the URL is the collection plus the task id, and the provider's own state
// vocabulary is mapped onto the envelope's.
func TestMediaCallService_PollVideoJob(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		answer     string
		wantURL    string
		wantStatus string
		wantURLOut string
	}{
		{
			name:       "the xAI poll reports the state without repeating the id",
			provider:   "xai",
			answer:     `{"status":"done","video":{"url":"https://cdn.x.ai/v.mp4"}}`,
			wantURL:    "https://api.x.ai/v1/videos/req-1",
			wantStatus: schema.VideoJobSucceeded,
			wantURLOut: "https://cdn.x.ai/v.mp4",
		},
		{
			name:       "the openrouter poll reports unsigned_urls",
			provider:   "openrouter",
			answer:     `{"id":"job-1","status":"completed","unsigned_urls":["https://cdn/v.mp4"]}`,
			wantURL:    "https://openrouter.ai/api/v1/videos/job-1",
			wantStatus: schema.VideoJobSucceeded,
			wantURLOut: "https://cdn/v.mp4",
		},
		{
			name:       "the agnes poll rebuilds the model the create encoded",
			provider:   "agnes",
			answer:     `{"id":"task_1","status":"completed","progress":100,"url":"https://cdn.agnes/v.mp4"}`,
			wantURL:    "https://apihub.agnes-ai.com/agnesapi?model_name=agnes-video-2.5-flash&video_id=video_1",
			wantStatus: schema.VideoJobSucceeded,
			wantURLOut: "https://cdn.agnes/v.mp4",
		},
		{
			name:       "the tokenrouter poll reads its envelope",
			provider:   "tokenrouter",
			answer:     `{"code":0,"message":"ok","data":{"task_id":"tr-1","status":"IN_PROGRESS","progress":55}}`,
			wantURL:    "https://api.tokenrouter.com/v1/video/generations/tr-1",
			wantStatus: schema.VideoJobInProgress,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(tc.answer)}

			taskID := "req-1"
			if tc.provider != "xai" {
				taskID = "job-1"
			}
			if tc.provider == "tokenrouter" {
				taskID = "tr-1"
			}
			if tc.provider == "agnes" {
				taskID = "dmlkZW9fMXxhZ25lcy12aWRlby0yLjUtZmxhc2g"
			}
			job, _, err := svc.PollVideoJob(context.Background(), tc.provider, "ep-"+tc.provider, taskID, "")
			if err != nil {
				t.Fatalf("PollVideoJob() error = %v", err)
			}
			if got := caller.requests[0].URL; got != tc.wantURL {
				t.Fatalf("poll URL = %q, want %q", got, tc.wantURL)
			}
			if got := caller.requests[0].Method; got != "GET" {
				t.Fatalf("poll method = %q, want GET", got)
			}
			if job.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", job.Status, tc.wantStatus)
			}
			if job.URL != tc.wantURLOut {
				t.Fatalf("url = %q, want %q", job.URL, tc.wantURLOut)
			}
			if job.TaskID != taskID {
				t.Fatalf("task id = %q, want %q: the envelope repeats the id the request carried", job.TaskID, taskID)
			}
			if len(router.pinned) != 1 || router.pinned[0] != "ep-"+tc.provider {
				t.Fatalf("pinned = %v, want the account that created the job", router.pinned)
			}
		})
	}
}

// TestMediaCallService_PollVideoJob_LeavesTheEndpointHealthAlone pins D5 of
// docs/PORT/011-PORT-VIDEO-ASYNC-JOB.md: a poll is a read of a job that already
// exists, so a stale task id answering 404 must not park a healthy key the way
// a rejected generation would.
func TestMediaCallService_PollVideoJob_LeavesTheEndpointHealthAlone(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"status":"processing","progress":10}`)}

	if _, _, err := svc.PollVideoJob(context.Background(), "xai", "ep-xai", "req-1", ""); err != nil {
		t.Fatalf("PollVideoJob() error = %v", err)
	}
	if router.successes != 0 {
		t.Fatalf("health successes = %d, want 0: a poll records no endpoint health", router.successes)
	}
	if len(router.failures) != 0 {
		t.Fatalf("health failures = %v, want none", router.failures)
	}
}

// TestMediaCallService_PollVideoJob_EscapesTheTaskID pins the URL rule the
// reference's own poll test carries (`id%20with%2Fslash`): a task id is one
// path segment, so an id carrying a slash cannot address a deeper path.
func TestMediaCallService_PollVideoJob_EscapesTheTaskID(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"status":"pending"}`)}

	if _, _, err := svc.PollVideoJob(context.Background(), "xai", "ep-xai", "id with/slash", ""); err != nil {
		t.Fatalf("PollVideoJob() error = %v", err)
	}
	if got := caller.requests[0].URL; got != "https://api.x.ai/v1/videos/id%20with%2Fslash" {
		t.Fatalf("poll URL = %q, want the escaped segment", got)
	}
}

// TestReadTokenRouterVideoJob_ReadsTheDocumentedProgress pins the progress
// member's real shape: TokenRouter's documented query example answers a
// percentage string ("100%"), and a plain number is read too. A job that
// answered is served either way, because the percentage is cosmetic.
func TestReadTokenRouterVideoJob_ReadsTheDocumentedProgress(t *testing.T) {
	cases := []struct {
		name     string
		progress string
		want     int
		hasWant  bool
	}{
		{name: "the documented percentage string", progress: `"100%"`, want: 100, hasWant: true},
		{name: "a bare percentage string", progress: `"55"`, want: 55, hasWant: true},
		{name: "a number", progress: `55`, want: 55, hasWant: true},
		{name: "null is no progress", progress: `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"code":"success","message":"","data":{"task_id":"task_1","status":"SUCCESS",` +
				`"result_url":"https://example.com/v.mp4","progress":` + tc.progress + `}}`
			job, err := readTokenRouterVideoJob([]byte(body), false)
			if err != nil {
				t.Fatalf("readTokenRouterVideoJob() error = %v, want a served job", err)
			}
			if job.Status != schema.VideoJobSucceeded || job.URL != "https://example.com/v.mp4" {
				t.Fatalf("job = %+v, want the succeeded job the envelope reports", job)
			}
			switch {
			case tc.hasWant && (job.Progress == nil || *job.Progress != tc.want):
				t.Fatalf("progress = %v, want %d", job.Progress, tc.want)
			case !tc.hasWant && job.Progress != nil:
				t.Fatalf("progress = %v, want none", *job.Progress)
			}
		})
	}
}
