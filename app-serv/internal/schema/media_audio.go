// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_audio.go
// @for       The speech and voice contracts of SPEC-API-001 §7.10.
// @uses      encoding/json, strings, internal/domain.
// @reason    §7.10 serves the OpenAI-ish shapes the reference handlers serve.
//
//	A speech answer is bytes unless the caller asks for JSON, so the
//	format decides the answer's shape here rather than in the handler.
//	The transcription contract, which arrives as multipart, lives in
//	media_transcription.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// DefaultSpeechVoice is the voice an OpenAI-compatible speech call uses when the
// client names none, matching the reference's default.
const DefaultSpeechVoice = "alloy"

// SpeechRequest is the body of POST /api/v1/audio/speech.
type SpeechRequest struct {
	Model          string   `json:"model" validate:"required,min=1,max=200"`
	Input          string   `json:"input" validate:"required,min=1,max=4096"`
	Voice          string   `json:"voice,omitempty" validate:"omitempty,max=64"`
	Language       string   `json:"language,omitempty" validate:"omitempty,max=16"`
	Speed          *float64 `json:"speed,omitempty" validate:"omitempty,gt=0,lte=4"`
	ResponseFormat string   `json:"response_format,omitempty" validate:"omitempty,oneof=mp3 opus aac flac wav pcm"`
}

// DecodeSpeechRequest decodes a speech body into its typed contract.
func DecodeSpeechRequest(raw []byte) (SpeechRequest, error) {
	var req SpeechRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return SpeechRequest{}, domain.NewValidationError("invalid request body: " + jsonErrorTail(err))
	}
	return req, nil
}

// SpeechBody is the payload an OpenAI-compatible speech endpoint expects.
type SpeechBody struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
}

// SpeechResponse is the JSON form of a speech answer, asked for with
// `?response_format=json`: the audio base64-encoded, because the route answers
// with the bytes themselves otherwise.
type SpeechResponse struct {
	Audio  string `json:"audio"`
	Format string `json:"format"`
}

// VoiceObject is one entry of the voice catalog.
type VoiceObject struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Lang   string `json:"lang,omitempty"`
	Gender string `json:"gender,omitempty"`
	Model  string `json:"model,omitempty"`
}

// VoiceList is the OpenAI list envelope the reference's voices route answers.
type VoiceList struct {
	Object string        `json:"object"`
	Data   []VoiceObject `json:"data"`
}

// SpeechFormat reports the audio format a speech answer carries, defaulting to
// mp3 the way the reference does.
func SpeechFormat(req SpeechRequest) string {
	if format := strings.TrimSpace(req.ResponseFormat); format != "" {
		return format
	}
	return "mp3"
}

// AudioContentType renders the media type for a speech format. The reference
// writes `audio/{format}` rather than a table of registered media types, and
// §7.10 pins its behaviour, so the spelling is preserved.
func AudioContentType(format string) string {
	return "audio/" + strings.TrimSpace(format)
}

// SpeechVoice reports the voice a speech call names, defaulting the way the
// reference does.
//
// The reference also accepts a voice as a third segment of the model string
// (`provider/model/voice`). That shorthand is deliberately not ported: a media
// model id may itself carry a slash (openrouter's `openai/gpt-4o-mini-tts`), so
// the same string would sometimes name a voice and sometimes name a model, and
// guessing wrong sends the upstream a model it cannot serve. The `voice` field
// is the OpenAI wire's own way of saying it.
func SpeechVoice(req SpeechRequest) string {
	if voice := strings.TrimSpace(req.Voice); voice != "" {
		return voice
	}
	return DefaultSpeechVoice
}
