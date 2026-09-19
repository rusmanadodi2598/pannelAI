// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_generation_test.go
// @for       HTTP tests for the §7.10 image, video, and search routes.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing,
//
//	internal/dataplane.
//
// @reason    Each of these routes answers with a normalized envelope rather
//
//	than the upstream's own body, so the shape a client parses is the
//	thing to pin — including the refusal the video route owes while no
//	provider declares the kind.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
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

// TestMediaHandler_Videos pins the refusal §7.10's video route owes a client
// while no provider declares the kind.
func TestMediaHandler_Videos(t *testing.T) {
	f, _ := newMediaHandlerFixture(t)
	rr := do(t, http.MethodPost, "/api/v1/videos/generations",
		`{"model":"openai/sora","prompt":"a cat"}`, f.Videos)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"PROVIDER_NOT_ROUTABLE"`) {
		t.Fatalf("body = %s, want the PROVIDER_NOT_ROUTABLE code", rr.Body.String())
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
