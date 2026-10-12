// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_vertex.go
// @for       The vertex (Veo) video dialect: the predictLongRunning creation, the fetchPredictOperation poll, and the request body translation between the two shapes.
// @uses      internal/dataplane, encoding/json, regexp, strings.
// @reason    Vertex translates both directions: the model, project, and location live in the path and the body is rebuilt as instances/parameters, so a path append cannot express it. The job id codec is split into media_video_vertex_id.go for the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-12
package service

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// vertexDataURLPattern reads the data URL form of a source asset: its MIME
// type and the base64 bytes after the comma.
var vertexDataURLPattern = regexp.MustCompile(`(?s)^data:([^;]+);base64,(.*)$`)

// buildVertexVideoCreation builds a Veo creation: the project and location are
// the endpoint's own service-account fields, and the model travels in the path
// rather than the body (videoProviders/vertex.js:150-155).
func buildVertexVideoCreation(call MediaCall, _ string, raw []byte) (dataplane.MediaRequest, error) {
	ref := call.Selection.Endpoint.ServiceAccount()
	model := strings.TrimSpace(call.UpstreamModel)
	if !vertexSegment(ref.ProjectID) || !vertexSegment(ref.Location) {
		return dataplane.MediaRequest{}, dataplane.ValidationError(
			"the vertex endpoint's project and location are not usable in a path")
	}
	if !vertexSegment(model) {
		return dataplane.MediaRequest{}, dataplane.ValidationError(
			"the vertex video model id is not usable in a path")
	}
	body, err := vertexVideoBody(raw)
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	target, err := dataplane.MediaPath(call.Target, "/v1/projects/"+ref.ProjectID+
		"/locations/"+ref.Location+"/publishers/google/models/"+model+":predictLongRunning")
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	return dataplane.MediaRequest{Method: "POST", URL: target, Body: body}, nil
}

// buildVertexVideoPoll builds a Veo poll: the job id decodes back to the
// operation name, whose model path is the fetchPredictOperation route
// (videoProviders/vertex.js:123-131).
func buildVertexVideoPoll(call MediaCall, taskID string, _ []byte) (dataplane.MediaRequest, error) {
	operation, err := decodeVertexJobID(taskID)
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	body, err := json.Marshal(vertexPollRequest{OperationName: operation})
	if err != nil {
		return dataplane.MediaRequest{}, dataplane.InternalError("the video poll request could not be built", err)
	}
	target, err := dataplane.MediaPath(call.Target, "/v1/"+vertexModelPath(operation)+":fetchPredictOperation")
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	return dataplane.MediaRequest{Method: "POST", URL: target, Body: body}, nil
}

// vertexPollRequest is the poll body: the operation the client's job id named.
type vertexPollRequest struct {
	OperationName string `json:"operationName"`
}

// vertexVideoRequest is the client's own body, read for the members the Vertex
// shape carries (videoProviders/vertex.js:60-87).
type vertexVideoRequest struct {
	Prompt         string          `json:"prompt"`
	Image          json.RawMessage `json:"image"`
	ImageURL       json.RawMessage `json:"image_url"`
	Video          json.RawMessage `json:"video"`
	N              *float64        `json:"n"`
	Duration       *float64        `json:"duration"`
	AspectRatio    string          `json:"aspect_ratio"`
	Resolution     string          `json:"resolution"`
	Seed           *int64          `json:"seed"`
	NegativePrompt string          `json:"negative_prompt"`
	StorageURI     string          `json:"storage_uri"`
	GenerateAudio  *bool           `json:"generate_audio"`
}

// vertexPredictParameters is the generation parameter block: the client's
// OpenAI-ish knobs under the model's own names (videoProviders/vertex.js:60-87).
// A pointer member is a knob that is absent from the body unless the client
// wrote it, so a zero is never sent in its place.
type vertexPredictParameters struct {
	SampleCount     *float64 `json:"sampleCount,omitempty"`
	DurationSeconds *float64 `json:"durationSeconds,omitempty"`
	AspectRatio     string   `json:"aspectRatio,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	Seed            *int64   `json:"seed,omitempty"`
	NegativePrompt  string   `json:"negativePrompt,omitempty"`
	StorageURI      string   `json:"storageUri,omitempty"`
	GenerateAudio   *bool    `json:"generateAudio,omitempty"`
}

// vertexPredictRequest is the body the model reference documents: one instance
// carrying the prompt and an optional source asset, beside the generation
// parameters.
type vertexPredictRequest struct {
	Instances  []vertexVideoInstance    `json:"instances"`
	Parameters *vertexPredictParameters `json:"parameters,omitempty"`
}

// vertexVideoInstance is one generation input.
type vertexVideoInstance struct {
	Prompt string          `json:"prompt,omitempty"`
	Image  json.RawMessage `json:"image,omitempty"`
	Video  json.RawMessage `json:"video,omitempty"`
}

// vertexVideoBody translates the client's body into the Vertex shape: the
// prompt becomes the instance, an image or image_url becomes its source asset,
// and the OpenAI-ish knobs become the model's own parameters.
func vertexVideoBody(raw []byte) ([]byte, error) {
	var in vertexVideoRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, dataplane.ValidationError("invalid request body: the video body could not be read")
	}
	instance := vertexVideoInstance{Prompt: in.Prompt}
	image, err := vertexImage(in.Image, in.ImageURL)
	if err != nil {
		return nil, err
	}
	instance.Image = image
	if len(in.Video) > 0 && !isJSONNull(in.Video) {
		instance.Video = in.Video
	}
	request := vertexPredictRequest{Instances: []vertexVideoInstance{instance}}
	if parameters := vertexParameters(in); parameters != (vertexPredictParameters{}) {
		request.Parameters = &parameters
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, dataplane.InternalError("the video request could not be built", err)
	}
	return encoded, nil
}

// vertexImage reads the source asset the reference accepts: a Vertex-native
// object passes through, a data URL becomes inline bytes, and any other string
// is a Cloud Storage URI.
func vertexImage(image, imageURL json.RawMessage) (json.RawMessage, error) {
	value := firstPresent(image, imageURL)
	if len(value) == 0 {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		if match := vertexDataURLPattern.FindStringSubmatch(text); match != nil {
			encoded, err := json.Marshal(map[string]string{
				"bytesBase64Encoded": match[2], "mimeType": match[1],
			})
			if err != nil {
				return nil, dataplane.InternalError("the video request could not be built", err)
			}
			return encoded, nil
		}
		encoded, err := json.Marshal(map[string]string{"gcsUri": text})
		if err != nil {
			return nil, dataplane.InternalError("the video request could not be built", err)
		}
		return encoded, nil
	}
	if strings.HasPrefix(strings.TrimSpace(string(value)), "{") {
		return value, nil
	}
	return nil, dataplane.ValidationError(
		"invalid request body: image must be a data URL, a Cloud Storage URI, or the Vertex image object")
}

// vertexParameters maps the OpenAI-ish knobs onto the model's own parameter
// names. An absent knob stays absent from the body rather than sent as a zero,
// so the empty parameter block means the client wrote none of them.
func vertexParameters(in vertexVideoRequest) vertexPredictParameters {
	var parameters vertexPredictParameters
	parameters.SampleCount = in.N
	parameters.DurationSeconds = in.Duration
	parameters.AspectRatio = in.AspectRatio
	parameters.Resolution = in.Resolution
	parameters.Seed = in.Seed
	parameters.NegativePrompt = in.NegativePrompt
	parameters.StorageURI = in.StorageURI
	parameters.GenerateAudio = in.GenerateAudio
	return parameters
}

// firstPresent reads the first member a client wrote with a value: a null is
// absent for every purpose here.
func firstPresent(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 && !isJSONNull(value) {
			return value
		}
	}
	return nil
}

// isJSONNull reports a member the client wrote as null.
func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}
