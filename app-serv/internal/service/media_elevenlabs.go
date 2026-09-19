// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_elevenlabs.go
// @for       The ElevenLabs speech adapter in the §7.10 media plane.
// @uses      internal/dataplane, internal/schema, encoding/json, net/url, strings.
// @reason    ElevenLabs names the voice in its path and reads the model from the
//
//	body, so it is the first adapter that shapes a URL of its own. Keeping
//	that beside its body means the shared pipeline still owns selection,
//	egress, health, and accounting.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// elevenLabsSpeechBody is the request ElevenLabs' text-to-speech endpoint
// accepts. The voice is a path segment rather than a body field.
type elevenLabsSpeechBody struct {
	Text          string                  `json:"text"`
	ModelID       string                  `json:"model_id"`
	VoiceSettings elevenLabsVoiceSettings `json:"voice_settings"`
}

// elevenLabsVoiceSettings fixes the two settings the reference sends.
type elevenLabsVoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
}

// elevenLabsSpeechPayload builds the provider's body and the URL its endpoint
// wants (`/text-to-speech/{voice_id}`).
//
// The voice is required. The reference reads it out of the model string when the
// caller names none, which this gateway deliberately does not do — §7.10 keeps
// the `voice` field as the wire's own way to say it — and a model id sent as a
// voice id would be a call the upstream rejects for a reason the caller cannot
// see.
func elevenLabsSpeechPayload(req schema.SpeechRequest, call MediaCall) (speechPayload, error) {
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		return speechPayload{}, dataplane.ValidationError("provider elevenlabs needs a voice for tts")
	}
	target, err := dataplane.MediaPath(call.Target, "/"+voice)
	if err != nil {
		return speechPayload{}, err
	}
	body, err := json.Marshal(elevenLabsSpeechBody{
		Text: req.Input, ModelID: call.UpstreamModel,
		VoiceSettings: elevenLabsVoiceSettings{Stability: 0.5, SimilarityBoost: 0.75},
	})
	return speechPayload{body: body, target: target}, err
}
