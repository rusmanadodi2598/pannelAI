// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_elevenlabs_test.go
// @for       The ElevenLabs speech adapter: body, voice path segment, declared
//
//	header, the missing-voice refusal, and the MP3 label.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
//	encoding/json, strings, testing.
//
// @reason    G5 ports one provider adapter at a time. ElevenLabs is the first
//
//	adapter that shapes a URL of its own, so these rows pin that the
//	voice lands in the path, that the model stays in the body, and that
//	the OpenAI-shaped control is unaffected.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_ElevenLabsSpeech pins the provider's request: the voice
// is a path segment, the model a body field, and the declared `xi-api-key`
// header carries the credential.
func TestMediaCallService_ElevenLabsSpeech(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	router.credential = provider.StaticKey("ep-elevenlabs", "key-el", "el-secret")
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("mp3-bytes")}

	req := schema.SpeechRequest{Model: "elevenlabs/eleven_turbo_v2_5", Input: "hello", Voice: "voice-42"}
	answer, call, err := svc.Speech(context.Background(), req, nil, "gky_el")
	if err != nil {
		t.Fatalf("Speech() error = %v", err)
	}
	if string(answer.Body) != "mp3-bytes" {
		t.Fatalf("answer = %q, want the upstream bytes", answer.Body)
	}
	request := caller.requests[0]
	if request.URL != "https://api.elevenlabs.io/v1/text-to-speech/voice-42" {
		t.Fatalf("URL = %q, want the voice as a path segment", request.URL)
	}
	if request.Headers["Xi-Api-Key"] != "el-secret" {
		t.Fatalf("xi-api-key = %q, want the endpoint credential", request.Headers["Xi-Api-Key"])
	}
	var body elevenLabsSpeechBody
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decoding ElevenLabs body: %v", err)
	}
	if body.ModelID != "eleven_turbo_v2_5" || body.Text != "hello" {
		t.Fatalf("body = %+v, want model/eleven_turbo_v2_5 text/hello", body)
	}
	if body.VoiceSettings.Stability != 0.5 || body.VoiceSettings.SimilarityBoost != 0.75 {
		t.Fatalf("voice settings = %+v, want the reference's values", body.VoiceSettings)
	}
	if got := call.SpeechOutputFormat(req); got != "mp3" {
		t.Fatalf("output format = %q, want mp3", got)
	}
}

// TestMediaCallService_ElevenLabsNeedsAVoice pins the refusal: the reference
// reads a voice out of the model string, and this gateway does not, so a call
// that names none is refused before anything is dialed rather than sent a model
// id as a voice id.
func TestMediaCallService_ElevenLabsNeedsAVoice(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)

	_, _, err := svc.Speech(context.Background(), schema.SpeechRequest{
		Model: "elevenlabs/eleven_multilingual_v2", Input: "hello",
	}, nil, "gky_el")
	if err == nil || !strings.Contains(err.Error(), "needs a voice") {
		t.Fatalf("error = %v, want a missing-voice refusal", err)
	}
	if len(caller.requests) != 0 {
		t.Fatalf("dialed %d times, want none for a refused request", len(caller.requests))
	}
}
