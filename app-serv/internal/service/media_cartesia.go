// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_cartesia.go
// @for       The Cartesia speech adapter in the §7.10 media plane.
// @uses      internal/schema, strings.
// @reason    Cartesia's TTS endpoint accepts a provider-specific JSON body and
//
//	needs a version header in addition to its X-API-Key. Keeping that
//	shape beside the other provider builders lets the shared pipeline
//	continue to own selection, egress, health, and accounting.
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

const cartesiaVersion = "2024-06-10"

// cartesiaSpeechBody is the request Cartesia's bytes endpoint accepts.
type cartesiaSpeechBody struct {
	ModelID      string               `json:"model_id"`
	Transcript   string               `json:"transcript"`
	Voice        *cartesiaVoice       `json:"voice,omitempty"`
	OutputFormat cartesiaOutputFormat `json:"output_format"`
}

// cartesiaVoice selects one documented Cartesia voice id. The pointer in the
// body is intentional: the reference omits voice when the caller names none.
type cartesiaVoice struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

// cartesiaOutputFormat fixes the binary format the reference adapter requests.
type cartesiaOutputFormat struct {
	Container  string `json:"container"`
	BitRate    int    `json:"bit_rate"`
	SampleRate int    `json:"sample_rate"`
}

// cartesiaSpeechRequest builds the provider-specific request and version header.
// Cartesia returns MP3 bytes regardless of the client's OpenAI response_format,
// so the provider adapter keeps one honest output label.
func cartesiaSpeechRequest(req schema.SpeechRequest, model string) (cartesiaSpeechBody, map[string]string) {
	body := cartesiaSpeechBody{
		ModelID: model, Transcript: req.Input,
		OutputFormat: cartesiaOutputFormat{Container: "mp3", BitRate: 128000, SampleRate: 44100},
	}
	if voice := strings.TrimSpace(req.Voice); voice != "" {
		body.Voice = &cartesiaVoice{Mode: "id", ID: voice}
	}
	return body, map[string]string{"Cartesia-Version": cartesiaVersion}
}
