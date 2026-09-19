// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_playht.go
// @for       The PlayHT speech adapter in the §7.10 media plane.
// @uses      internal/schema, strings.
// @reason    PlayHT's body names the voice and the engine in its own fields and
//
//	its API answers audio bytes, so the adapter is the request shape
//	alone. The credential is a `userId:apiKey` pair the kind's block
//	declares, which MediaTarget splits into the two headers.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// playHTDefaultVoice is the reference's default voice manifest. PlayHT requires
// a voice, and a made-up id would be a call the upstream rejects; this is the
// one the reference ships.
const playHTDefaultVoice = "s3://voice-cloning-zero-shot/d9ff78ba-d016-47f6-b0ef-dd630f59414e/female-cs/manifest.json"

// playhtSpeechBody is the request PlayHT's streaming endpoint accepts. The route
// asks for mp3 and the reference's speed, so the label it reports stays honest.
type playhtSpeechBody struct {
	Text         string  `json:"text"`
	Voice        string  `json:"voice"`
	VoiceEngine  string  `json:"voice_engine"`
	OutputFormat string  `json:"output_format"`
	Speed        float64 `json:"speed"`
}

// playhtSpeechRequest builds the provider's body.
func playhtSpeechRequest(req schema.SpeechRequest, model string) playhtSpeechBody {
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = playHTDefaultVoice
	}
	return playhtSpeechBody{
		Text: req.Input, Voice: voice, VoiceEngine: model,
		OutputFormat: "mp3", Speed: 1,
	}
}
