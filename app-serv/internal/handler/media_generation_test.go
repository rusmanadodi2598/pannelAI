// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_generation_test.go
// @for       HTTP tests for the §7.10 image, video, and search routes.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing, internal/dataplane.
// @reason    Each of these routes answers with a normalized envelope rather than the upstream's own body, so the shape a client parses is the thing to pin, including the refusal the video route owes while no provider declares the kind.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-19
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// TestMediaHandler_Images pins the generation route end to end.
func TestMediaHandler_Images(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(
		`{"created":7,"data":[{"url":"https://img.example.com/a.png"}]}`)}

	rr := do(t, http.MethodPost, "/api/v1/images/generations",
		`{"model":"openai/gpt-image-1","prompt":"a cat","n":1}`, f.Images)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["created"] != float64(7) {
		t.Fatalf("created = %v, want the upstream value", body["created"])
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data = %v, want one asset", body["data"])
	}
}

// TestMediaHandler_Videos pins the async job's creation: the normalized
// envelope comes back, and the endpoint the job was created on travels in the
// header the client pins its polls with.
func TestMediaHandler_Videos(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"request_id":"req-1"}`)}

	rr := do(t, http.MethodPost, "/api/v1/videos/generations",
		`{"model":"xai/grok-imagine-video","prompt":"a paper boat","duration":8}`, f.Videos)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["task_id"] != "req-1" || body["status"] != "queued" {
		t.Fatalf("body = %v, want the created job", body)
	}
	if got := rr.Header().Get(VideoEndpointHeader); got != "ep-xai" {
		t.Fatalf("header %s = %q, want the created endpoint", VideoEndpointHeader, got)
	}
	if len(caller.requests) != 1 || caller.requests[0].URL != "https://api.x.ai/v1/videos/generations" {
		t.Fatalf("upstream requests = %+v, want one creation on the xAI collection", caller.requests)
	}
}

// TestMediaHandler_VideoJob pins the poll: the task id comes from the path, the
// provider from the query, and the answer is the same envelope the creation
// served.
func TestMediaHandler_VideoJob(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(
		`{"status":"done","video":{"url":"https://cdn.x.ai/v.mp4"}}`)}

	rr := do(t, http.MethodGet, "/api/v1/videos/req-1?provider=xai&endpoint=ep-xai",
		"", withPathValue(f.VideoJob, "task_id", "req-1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["task_id"] != "req-1" || body["status"] != "succeeded" || body["url"] != "https://cdn.x.ai/v.mp4" {
		t.Fatalf("body = %v, want the polled job", body)
	}
	if len(caller.requests) != 1 || caller.requests[0].URL != "https://api.x.ai/v1/videos/req-1" {
		t.Fatalf("upstream requests = %+v, want one poll of the task", caller.requests)
	}
}

// TestMediaHandler_VideoJob_RequiresTheProvider pins the one required query
// parameter: a poll carries no model, so nothing else could name the provider.
func TestMediaHandler_VideoJob_RequiresTheProvider(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	rr := do(t, http.MethodGet, "/api/v1/videos/req-1", "",
		withPathValue(f.VideoJob, "task_id", "req-1"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"VALIDATION_ERROR"`) ||
		!strings.Contains(rr.Body.String(), "Provider") {
		t.Fatalf("body = %s, want the provider refusal", rr.Body.String())
	}
	if len(caller.requests) != 0 {
		t.Fatalf("upstream requests = %+v, want none", caller.requests)
	}
}

// TestMediaHandler_VideoJob_RejectsAnOversizedPoll pins the bounds on the poll's
// query and path: an id too long to be a provider, task, or endpoint is refused
// before an upstream URL is built from it.
func TestMediaHandler_VideoJob_RejectsAnOversizedPoll(t *testing.T) {
	long := strings.Repeat("x", 257)
	cases := []struct {
		name   string
		target string
		taskID string
		field  string
	}{
		{name: "an over-long provider", target: "/api/v1/videos/req-1?provider=" + strings.Repeat("p", 65), taskID: "req-1", field: "Provider"},
		{name: "an over-long endpoint", target: "/api/v1/videos/req-1?provider=xai&endpoint=" + strings.Repeat("e", 65), taskID: "req-1", field: "Endpoint"},
		{name: "an over-long task id", target: "/api/v1/videos/" + long + "?provider=xai", taskID: long, field: "TaskID"},
		{name: "a missing task id", target: "/api/v1/videos/?provider=xai", taskID: "", field: "TaskID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, caller := newMediaHandlerFixture(t)
			rr := do(t, http.MethodGet, tc.target, "", withPathValue(f.VideoJob, "task_id", tc.taskID))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tc.field) {
				t.Fatalf("body = %s, want a refusal naming %s", rr.Body.String(), tc.field)
			}
			if len(caller.requests) != 0 {
				t.Fatalf("upstream requests = %+v, want none", caller.requests)
			}
		})
	}
}

// TestMediaHandler_Search pins the search route end to end.
func TestMediaHandler_Search(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(
		`{"web":{"results":[{"title":"Go","url":"https://go.dev","description":"the language"}]}}`)}

	rr := do(t, http.MethodPost, "/api/v1/search",
		`{"provider":"brave-search","query":"golang generics"}`, f.Search)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["provider"] != "brave-search" || body["query"] != "golang generics" {
		t.Fatalf("identity = %v/%v, want the provider and the query", body["provider"], body["query"])
	}
	results, _ := body["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want one", body["results"])
	}
}

// TestMediaHandler_SearchDecodesTheNormalizedEnvelope pins the answer shape a
// client parses, including the metrics the route reports.
func TestMediaHandler_SearchDecodesTheNormalizedEnvelope(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"results":[{"title":"Go","url":"https://go.dev"}]}`)}

	rr := do(t, http.MethodPost, "/api/v1/search", `{"provider":"brave-search","query":"go"}`, f.Search)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var decoded struct {
		Results []struct {
			Position int `json:"position"`
		} `json:"results"`
		Usage struct {
			QueriesUsed int `json:"queries_used"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	if len(decoded.Results) != 1 || decoded.Results[0].Position != 1 {
		t.Fatalf("results = %+v, want one result at position 1", decoded.Results)
	}
	if decoded.Usage.QueriesUsed != 1 {
		t.Fatalf("usage = %+v, want one query", decoded.Usage)
	}
}
