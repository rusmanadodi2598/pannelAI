// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_vertex_test.go
// @for       The vertex (Veo) video dialect: the predictLongRunning creation, the fetchPredictOperation poll, and the body translation between the two shapes.
// @uses      internal/dataplane, internal/schema, context, encoding/base64, encoding/json, strings, testing.
// @reason    Vertex does not speak the OpenAI-ish video shape: the model, project, and location live in the path and the body is translated, so a wrong builder is a billable POST to an address nothing answers. The id codec's own tests live in media_video_vertex_id_test.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// vertexModel and vertexOperation are the model a creation addresses and the
// operation name it answers with.
const (
	vertexModel     = "veo-3.1-generate-preview"
	vertexOperation = "projects/demo-project/locations/us-central1/publishers/google/models/" + vertexModel + "/operations/op-1"
)

// vertexJobID spells an operation name the way the creation answers it.
func vertexJobID(operation string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(operation))
}

// TestMediaCallService_GenerateVideoJob_VertexPlan pins the creation: the URL
// names the project, location, and model, the body is translated to the
// instances/parameters shape, and the answer's operation name becomes the job
// id the client polls with.
func TestMediaCallService_GenerateVideoJob_VertexPlan(t *testing.T) {
	minter := &stubVertexToken{token: "ya29.minted"}
	svc, caller, _ := vertexVideoFixture(t, minter)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"name":"` + vertexOperation + `","done":false}`)}

	raw := []byte(`{"model":"vertex/` + vertexModel + `","prompt":"a paper boat","duration":8,"aspect_ratio":"16:9","n":2}`)
	job, _, err := svc.GenerateVideoJob(context.Background(), schema.VideoRequest{
		Model: "vertex/" + vertexModel, Prompt: "a paper boat",
	}, raw, "")
	if err != nil {
		t.Fatalf("GenerateVideoJob() error = %v", err)
	}
	if job.TaskID != vertexJobID(vertexOperation) {
		t.Fatalf("task id = %q, want the encoded operation name", job.TaskID)
	}
	if job.Status != schema.VideoJobQueued {
		t.Fatalf("status = %q, want queued: the operation is not done", job.Status)
	}
	request := caller.requests[0]
	wantURL := "https://aiplatform.googleapis.com/v1/projects/demo-project/locations/us-central1/publishers/google/models/" + vertexModel + ":predictLongRunning"
	if request.URL != wantURL {
		t.Fatalf("create URL = %q, want %q", request.URL, wantURL)
	}
	if request.Method != "POST" {
		t.Fatalf("create method = %q, want POST", request.Method)
	}
	if got := request.Headers["Authorization"]; got != "Bearer ya29.minted" {
		t.Fatalf("Authorization = %q, want the minted bearer", got)
	}
	var sent struct {
		Instances  []map[string]any `json:"instances"`
		Parameters map[string]any   `json:"parameters"`
	}
	if err := json.Unmarshal(request.Body, &sent); err != nil {
		t.Fatalf("the upstream body is not JSON: %v", err)
	}
	if len(sent.Instances) != 1 || sent.Instances[0]["prompt"] != "a paper boat" {
		t.Fatalf("instances = %+v, want one carrying the prompt", sent.Instances)
	}
	if sent.Parameters["durationSeconds"] != float64(8) || sent.Parameters["aspectRatio"] != "16:9" || sent.Parameters["sampleCount"] != float64(2) {
		t.Fatalf("parameters = %+v, want the client's own knobs", sent.Parameters)
	}
	// The model member travelled in the path instead.
	if strings.Contains(string(request.Body), `"model"`) {
		t.Fatalf("upstream body = %s, want no model member", request.Body)
	}
}

// TestMediaCallService_PollVideoJob_VertexPlan pins the poll: the job id
// decodes back to the operation name, the request is a POST carrying it, and
// the operation's own state becomes the envelope's.
func TestMediaCallService_PollVideoJob_VertexPlan(t *testing.T) {
	minter := &stubVertexToken{token: "ya29.minted"}
	svc, caller, router := vertexVideoFixture(t, minter)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(
		`{"name":"` + vertexOperation + `","done":true,"response":{"videos":[{"gcsUri":"https://storage.googleapis.com/bucket/v.mp4","mimeType":"video/mp4"}]}}`)}

	job, _, err := svc.PollVideoJob(context.Background(), "vertex", "ep-vertex", vertexJobID(vertexOperation), "")
	if err != nil {
		t.Fatalf("PollVideoJob() error = %v", err)
	}
	request := caller.requests[0]
	wantURL := "https://aiplatform.googleapis.com/v1/projects/demo-project/locations/us-central1/publishers/google/models/" + vertexModel + ":fetchPredictOperation"
	if request.URL != wantURL {
		t.Fatalf("poll URL = %q, want %q", request.URL, wantURL)
	}
	if request.Method != "POST" {
		t.Fatalf("poll method = %q, want POST", request.Method)
	}
	if got := string(request.Body); got != `{"operationName":"`+vertexOperation+`"}` {
		t.Fatalf("poll body = %s, want the operation name", got)
	}
	if job.Status != schema.VideoJobSucceeded || job.URL != "https://storage.googleapis.com/bucket/v.mp4" {
		t.Fatalf("job = %+v, want the succeeded video", job)
	}
	if job.TaskID != vertexJobID(vertexOperation) {
		t.Fatalf("task id = %q, want the id the request carried", job.TaskID)
	}
	if len(router.pinned) != 1 || router.pinned[0] != "ep-vertex" {
		t.Fatalf("pinned = %v, want the account that created the job", router.pinned)
	}
}

// TestMediaCallService_PollVideoJob_VertexRefusesABadJobID pins that a job id
// the codec will not decode stops the poll before any request.
func TestMediaCallService_PollVideoJob_VertexRefusesABadJobID(t *testing.T) {
	minter := &stubVertexToken{token: "ya29.minted"}
	svc, caller, _ := vertexVideoFixture(t, minter)

	_, _, err := svc.PollVideoJob(context.Background(), "vertex", "ep-vertex", vertexJobID("hello"), "")
	if err == nil || !strings.Contains(err.Error(), "job id") {
		t.Fatalf("error = %v, want the job id refusal", err)
	}
	if len(caller.requests) != 0 {
		t.Fatalf("outbound calls = %d, want none", len(caller.requests))
	}
}

// TestVertexVideoBody_ReadsTheSourceImage pins the three source-asset shapes
// the reference accepts: a data URL becomes inline bytes, any other string is
// a Cloud Storage URI, and the Vertex-native object passes through. image wins
// over image_url when both are written.
func TestVertexVideoBody_ReadsTheSourceImage(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "a data URL becomes inline bytes",
			raw:  `{"prompt":"p","image":"data:image/png;base64,QUJD"}`,
			want: `{"bytesBase64Encoded":"QUJD","mimeType":"image/png"}`,
		},
		{
			name: "another string is a storage URI",
			raw:  `{"prompt":"p","image_url":"gs://bucket/first.png"}`,
			want: `{"gcsUri":"gs://bucket/first.png"}`,
		},
		{
			name: "the Vertex object passes through",
			raw:  `{"prompt":"p","image":{"bytesBase64Encoded":"QUJD","mimeType":"image/jpeg"}}`,
			want: `{"bytesBase64Encoded":"QUJD","mimeType":"image/jpeg"}`,
		},
		{
			name: "image wins over image_url",
			raw:  `{"prompt":"p","image":"gs://bucket/first.png","image_url":"gs://bucket/second.png"}`,
			want: `{"gcsUri":"gs://bucket/first.png"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := vertexVideoBody([]byte(tc.raw))
			if err != nil {
				t.Fatalf("vertexVideoBody() error = %v", err)
			}
			var sent struct {
				Instances []struct {
					Image json.RawMessage `json:"image"`
				} `json:"instances"`
			}
			if err := json.Unmarshal(body, &sent); err != nil {
				t.Fatalf("the translated body is not JSON: %v", err)
			}
			if len(sent.Instances) != 1 || string(sent.Instances[0].Image) != tc.want {
				t.Fatalf("image = %s, want %s", sent.Instances[0].Image, tc.want)
			}
		})
	}
}

// TestVertexVideoBody_RefusesAnImageItCannotRead pins that a shape neither
// string nor object is refused rather than dropped: a client that sent an
// asset and got a text-to-video result would read it as the model's choice.
func TestVertexVideoBody_RefusesAnImageItCannotRead(t *testing.T) {
	if _, err := vertexVideoBody([]byte(`{"prompt":"p","image":42}`)); err == nil {
		t.Fatal("vertexVideoBody() = nil error, want the image refusal")
	}
}

// TestMediaCallService_GenerateVideoJob_VertexRefusesAPathInTheModel pins the
// one client-controlled value that lands in the creation path: a model id that
// is not a plain segment is refused rather than spliced.
func TestMediaCallService_GenerateVideoJob_VertexRefusesAPathInTheModel(t *testing.T) {
	minter := &stubVertexToken{token: "ya29.minted"}
	svc, caller, _ := vertexVideoFixture(t, minter)

	_, _, err := svc.GenerateVideoJob(context.Background(), schema.VideoRequest{
		Model: "vertex/..", Prompt: "a paper boat",
	}, []byte(`{"model":"vertex/..","prompt":"a paper boat"}`), "")
	if err == nil {
		t.Fatal("GenerateVideoJob() error = nil, want the model refusal")
	}
	if len(caller.requests) != 0 {
		t.Fatalf("outbound calls = %d, want none", len(caller.requests))
	}
}
