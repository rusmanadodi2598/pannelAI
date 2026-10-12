// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_vertex_read.go
// @for       Reading one Vertex operation into the normalized job envelope.
// @uses      internal/dataplane, internal/schema, encoding/json, strings.
// @reason    Vertex reports a job's state as booleans and carries the asset in two shapes, a Cloud Storage URI or inline bytes, so the mapping is its own reader rather than a vocabulary table. It lives apart from media_video_read.go for the AGENTS.md §1.1 budget.
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

// vertexVideoAnswer is the operation a Veo call answers with: `name` is the
// resource path the job id encodes, `done` is the state, and a completed
// generation carries its samples under either of the two members the reference
// reads (videoProviders/vertex.js:90-109).
type vertexVideoAnswer struct {
	Name     string          `json:"name"`
	Done     bool            `json:"done"`
	Error    json.RawMessage `json:"error"`
	Response *struct {
		Videos                []vertexVideoSample `json:"videos"`
		GenerateVideoResponse *struct {
			GeneratedSamples []vertexVideoSample `json:"generatedSamples"`
		} `json:"generateVideoResponse"`
	} `json:"response"`
}

// vertexVideoSample is one generated asset, in either shape Vertex answers it:
// a storage address, or the bytes themselves for a call without a storage URI.
type vertexVideoSample struct {
	GCSURI             string `json:"gcsUri"`
	URI                string `json:"uri"`
	BytesBase64Encoded string `json:"bytesBase64Encoded"`
	MimeType           string `json:"mimeType"`
	Video              *struct {
		URI                string `json:"uri"`
		BytesBase64Encoded string `json:"bytesBase64Encoded"`
		MimeType           string `json:"mimeType"`
	} `json:"video"`
}

// readVertexVideoJob reads an operation into the envelope. An answer that
// names no operation is a failure rather than an empty job: the client's next
// move is to poll, and the job id is what it polls with.
func readVertexVideoJob(body []byte, requireTask bool) (schema.VideoJob, error) {
	var answer vertexVideoAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return schema.VideoJob{}, unreadableVideoAnswer(err)
	}
	if strings.TrimSpace(answer.Name) == "" {
		return schema.VideoJob{}, dataplane.UpstreamRejected(200,
			"the video upstream answered no operation to read")
	}
	job := schema.VideoJob{TaskID: encodeVertexJobID(answer.Name)}
	switch {
	case len(answer.Error) > 0:
		job.Status = schema.VideoJobFailed
		job.Error = videoErrorMessage(answer.Error)
	case !answer.Done:
		job.Status = schema.VideoJobQueued
	default:
		job.Status = schema.VideoJobSucceeded
		if sample := answer.firstSample(); sample != nil {
			job.URL = sample.url()
			job.B64JSON = sample.b64()
		}
	}
	return finishVideoJob(job, requireTask)
}

// firstSample picks the asset a completed operation carries, from either
// member shape.
func (a vertexVideoAnswer) firstSample() *vertexVideoSample {
	if a.Response == nil {
		return nil
	}
	if len(a.Response.Videos) > 0 {
		return &a.Response.Videos[0]
	}
	if a.Response.GenerateVideoResponse != nil && len(a.Response.GenerateVideoResponse.GeneratedSamples) > 0 {
		return &a.Response.GenerateVideoResponse.GeneratedSamples[0]
	}
	return nil
}

// url reads the asset's address, wherever the shape carries it.
func (s vertexVideoSample) url() string {
	if s.GCSURI != "" {
		return s.GCSURI
	}
	if s.Video != nil && s.Video.URI != "" {
		return s.Video.URI
	}
	return s.URI
}

// b64 reads the asset's inline bytes, which is what a call without a storage
// URI gets.
func (s vertexVideoSample) b64() string {
	if s.BytesBase64Encoded != "" {
		return s.BytesBase64Encoded
	}
	if s.Video != nil {
		return s.Video.BytesBase64Encoded
	}
	return ""
}
