// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_local_tts.go
// @for       The two self-hosted speech adapters (Coqui, Tortoise) in the §7.10
//
//	media plane.
//
// @uses      internal/schema, strings.
// @reason    Both providers run on the operator's own machine, take no
//
//	credential, accept a small JSON body, and answer WAV bytes, so they
//	share one file: the difference is which field names the speaker, and
//	keeping that pair side by side is what makes the sameness visible
//	instead of looking like two adapters that happen to agree.
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

// tortoiseDefaultVoice is the reference's default speaker name.
const tortoiseDefaultVoice = "random"

// coquiSpeechBody is the request a Coqui TTS server accepts. The server picks
// its model at startup, so the route's model string is not part of the body.
type coquiSpeechBody struct {
	Text      string `json:"text"`
	SpeakerID string `json:"speaker_id,omitempty"`
}

// tortoiseSpeechBody is the request a Tortoise TTS server accepts.
type tortoiseSpeechBody struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
}

// coquiSpeechRequest builds the provider's body, naming a speaker only when the
// caller asked for one: Coqui's own default applies otherwise.
func coquiSpeechRequest(req schema.SpeechRequest) coquiSpeechBody {
	return coquiSpeechBody{Text: req.Input, SpeakerID: strings.TrimSpace(req.Voice)}
}

// tortoiseSpeechRequest builds the provider's body, which always carries a voice.
func tortoiseSpeechRequest(req schema.SpeechRequest) tortoiseSpeechBody {
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = tortoiseDefaultVoice
	}
	return tortoiseSpeechBody{Text: req.Input, Voice: voice}
}
