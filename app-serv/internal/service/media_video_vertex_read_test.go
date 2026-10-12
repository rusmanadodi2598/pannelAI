// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_vertex_read_test.go
// @for       Reading one Vertex operation into the normalized job envelope: the two state booleans, the error member, and the two shapes a completed generation carries its asset in.
// @uses      internal/schema, strings, testing.
// @reason    Vertex reports a job's state as booleans rather than a vocabulary, so there is nothing to map and everything to pin: a misread `done` reports a running job as finished, and a sample shape the reader does not know silently drops the asset the client paid for.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestReadVertexVideoJob pins the operation-to-envelope mapping.
func TestReadVertexVideoJob(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		requireTask bool
		want        schema.VideoJob
		wantErr     bool
	}{
		{
			name: "a running operation is a queued job",
			body: `{"name":"` + vertexOperation + `","done":false}`,
			want: schema.VideoJob{TaskID: vertexJobID(vertexOperation), Status: schema.VideoJobQueued},
		},
		{
			name: "a completed operation with a storage URI is succeeded",
			body: `{"name":"` + vertexOperation + `","done":true,"response":{"videos":[{"gcsUri":"https://storage.googleapis.com/bucket/v.mp4","mimeType":"video/mp4"}]}}`,
			want: schema.VideoJob{
				TaskID: vertexJobID(vertexOperation), Status: schema.VideoJobSucceeded,
				URL: "https://storage.googleapis.com/bucket/v.mp4",
			},
		},
		{
			name: "a completed operation carries generatedSamples too",
			body: `{"name":"` + vertexOperation + `","done":true,"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"uri":"https://storage.googleapis.com/bucket/s.mp4"}}]}}}`,
			want: schema.VideoJob{
				TaskID: vertexJobID(vertexOperation), Status: schema.VideoJobSucceeded,
				URL: "https://storage.googleapis.com/bucket/s.mp4",
			},
		},
		{
			name: "a call without a storage URI answers inline bytes",
			body: `{"name":"` + vertexOperation + `","done":true,"response":{"videos":[{"bytesBase64Encoded":"QUJD","mimeType":"video/mp4"}]}}`,
			want: schema.VideoJob{
				TaskID: vertexJobID(vertexOperation), Status: schema.VideoJobSucceeded,
				B64JSON: "QUJD",
			},
		},
		{
			name: "an error member is a failed job",
			body: `{"name":"` + vertexOperation + `","error":{"code":3,"message":"the prompt was refused"}}`,
			want: schema.VideoJob{
				TaskID: vertexJobID(vertexOperation), Status: schema.VideoJobFailed,
				Error: "the prompt was refused",
			},
		},
		{
			name: "an operation that is done without samples is still succeeded",
			body: `{"name":"` + vertexOperation + `","done":true}`,
			want: schema.VideoJob{TaskID: vertexJobID(vertexOperation), Status: schema.VideoJobSucceeded},
		},
		{
			name:    "an answer that names no operation is refused",
			body:    `{"done":true}`,
			wantErr: true,
		},
		{
			name:        "a creation answer that names no operation is refused",
			body:        `{"done":false}`,
			requireTask: true,
			wantErr:     true,
		},
		{
			name:    "a body that is not JSON is refused",
			body:    `<html>proxy error</html>`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readVertexVideoJob([]byte(tc.body), tc.requireTask)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("readVertexVideoJob() = %+v, want a refusal", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("readVertexVideoJob() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("readVertexVideoJob() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestReadVertexVideoJob_RefusesAnAnswerWithoutAnOperation pins that the
// refusal names the operation rather than reporting an empty job the client
// cannot poll.
func TestReadVertexVideoJob_RefusesAnAnswerWithoutAnOperation(t *testing.T) {
	_, err := readVertexVideoJob([]byte(`{"done":false}`), false)
	if err == nil || !strings.Contains(err.Error(), "operation") {
		t.Fatalf("error = %v, want the operation refusal", err)
	}
}
