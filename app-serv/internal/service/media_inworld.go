// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_inworld.go
// @for       The Inworld speech adapter in the §7.10 media plane.
// @uses      internal/dataplane, internal/schema, encoding/base64, encoding/json,
//
//	strings.
//
// @reason    Inworld authenticates with Basic and answers base64 audio inside
//
//	JSON, so both its body and its answer need the adapter. Keeping
//	them together lets the shared pipeline own selection, egress,
//	health, and accounting.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// inworldDefaultVoice is the reference's default when the caller names none.
const inworldDefaultVoice = "Alex"

// inworldSpeechBody is the request Inworld's voice endpoint accepts.
type inworldSpeechBody struct {
	Text        string             `json:"text"`
	VoiceID     string             `json:"voiceId"`
	ModelID     string             `json:"modelId"`
	AudioConfig inworldAudioConfig `json:"audioConfig"`
}

// inworldAudioConfig fixes the container the route reports.
type inworldAudioConfig struct {
	AudioEncoding string `json:"audioEncoding"`
}

// inworldSpeechRequest builds the provider's body. The credential is Basic,
// which the kind's block declares and MediaTarget places.
func inworldSpeechRequest(req schema.SpeechRequest, model string) inworldSpeechBody {
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = inworldDefaultVoice
	}
	return inworldSpeechBody{
		Text: req.Input, VoiceID: voice, ModelID: model,
		AudioConfig: inworldAudioConfig{AudioEncoding: "MP3"},
	}
}

// inworldSpeechAnswer decodes the base64 audio an Inworld answer carries. A
// 200 without `audioContent` is a failure the client would otherwise receive as
// a silent empty file.
func inworldSpeechAnswer(status int, body []byte) ([]byte, error) {
	var answer struct {
		AudioContent string `json:"audioContent"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, dataplane.InternalError("the speech answer could not be read", err)
	}
	audio, err := base64.StdEncoding.DecodeString(strings.TrimSpace(answer.AudioContent))
	if err != nil {
		return nil, dataplane.UpstreamRejected(status, "the speech upstream returned an unreadable audio payload")
	}
	if len(audio) == 0 {
		return nil, dataplane.UpstreamRejected(status, "the speech upstream returned no audio")
	}
	return audio, nil
}
