// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_speech.go
// @for       How one §7.10 speech call is shaped: the per-format request
//
//	builders, the answer readers, and the output label.
//
// @uses      internal/dataplane, internal/registry, internal/schema,
//
//	encoding/json, strings.
//
// @reason    Every speech provider answers in its own shape, so the route needs
//
//	one place that says which payload each format expects and how its
//	answer is read. Keeping the dispatch here means a new adapter is one
//	case plus its own file, and media_audio.go keeps the routes inside
//	the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// mediaFormat is a provider's declared format in the spelling the switches
// compare, so one file lowercasing it cannot drift from another.
func mediaFormat(media registry.MediaConfig) string {
	return strings.ToLower(strings.TrimSpace(media.Format))
}

// speechPayload is one provider's outbound shape: the body, any headers the
// target does not carry, and a URL of its own when the endpoint names the voice
// or the model in its path.
type speechPayload struct {
	body    []byte
	headers map[string]string
	target  string
}

// speechRequest builds the request POST /audio/speech sends to the selected
// provider.
func speechRequest(call MediaCall, req schema.SpeechRequest) (dataplane.MediaRequest, error) {
	payload, err := buildSpeechPayload(call, req)
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	return dataplane.MediaRequest{
		Method: "POST", URL: payload.target, Headers: payload.headers, Body: payload.body,
	}, nil
}

// buildSpeechPayload dispatches on the format the provider's block declares. An
// empty declaration and `openai` are the OpenAI body this route serves natively;
// every other format is a provider-specific builder in its own file.
func buildSpeechPayload(call MediaCall, req schema.SpeechRequest) (speechPayload, error) {
	var payload speechPayload
	var err error
	switch mediaFormat(call.Media) {
	case "nvidia-tts":
		payload.body, err = json.Marshal(nvidiaSpeechRequest(req, call.UpstreamModel))
	case "cartesia":
		var headers map[string]string
		var body cartesiaSpeechBody
		body, headers = cartesiaSpeechRequest(req, call.UpstreamModel)
		payload.headers = headers
		payload.body, err = json.Marshal(body)
	case "elevenlabs":
		payload, err = elevenLabsSpeechPayload(req, call)
	case "minimax-tts":
		payload.body, err = json.Marshal(minimaxSpeechRequest(req, call.UpstreamModel))
	case "inworld":
		payload.body, err = json.Marshal(inworldSpeechRequest(req, call.UpstreamModel))
	case "playht":
		payload.headers = map[string]string{"Accept": "audio/mpeg"}
		payload.body, err = json.Marshal(playhtSpeechRequest(req, call.UpstreamModel))
	case "coqui":
		payload.body, err = json.Marshal(coquiSpeechRequest(req))
	case "tortoise":
		payload.body, err = json.Marshal(tortoiseSpeechRequest(req))
	case "gemini-tts":
		payload, err = geminiSpeechPayload(req, call)
	default:
		payload.body, err = json.Marshal(schema.SpeechBody{
			Model:          call.UpstreamModel,
			Input:          req.Input,
			Voice:          schema.SpeechVoice(req),
			ResponseFormat: strings.TrimSpace(req.ResponseFormat),
			Speed:          req.Speed,
		})
	}
	if err != nil {
		return speechPayload{}, speechBuildError(err)
	}
	return payload, nil
}

// speechBuildError keeps an adapter's own refusal — a voice the provider needs,
// a target that cannot be shaped — and names a marshal failure as the request's
// own, so a 400 stays a 400 instead of becoming an internal error.
func speechBuildError(err error) error {
	if dataplane.AsError(err) != nil {
		return err
	}
	return dataplane.InternalError("the speech request could not be built", err)
}

// speechReader reports how a provider's speech answer is read into the bytes the
// route returns. A nil reader means the answer already is those bytes; the
// providers that answer base64, hex, or raw PCM inside JSON name their reader
// here, and one that answers an empty body is refused rather than served.
func speechReader(call MediaCall) mediaAnswerReader {
	switch mediaFormat(call.Media) {
	case "minimax-tts":
		return minimaxSpeechAnswer
	case "inworld":
		return inworldSpeechAnswer
	case "gemini-tts":
		return geminiSpeechAnswer
	case "elevenlabs", "playht", "coqui", "tortoise":
		return requireSpeechAudio
	default:
		return nil
	}
}

// requireSpeechAudio refuses a 2xx answer that carries no audio at all: an
// empty body is not a served call, and recording it as one would hide a
// provider failure the client already saw.
func requireSpeechAudio(_ int, body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, dataplane.UpstreamRejected(200, "the speech upstream returned no audio")
	}
	return body, nil
}

// SpeechOutputFormat maps the provider's response bytes to the route's output
// label. A provider-specific adapter returns one honest format because it asks
// its upstream for exactly that; the OpenAI-shaped path follows the client's
// requested format or its mp3 default.
func (c MediaCall) SpeechOutputFormat(req schema.SpeechRequest) string {
	switch mediaFormat(c.Media) {
	case "nvidia-tts", "coqui", "tortoise", "gemini-tts":
		return "wav"
	case "cartesia", "elevenlabs", "minimax-tts", "inworld", "playht":
		return "mp3"
	default:
		return schema.SpeechFormat(req)
	}
}
