// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_inworld_test.go
// @for       The Inworld speech adapter: nested body, Basic credential, and the
//
//	base64 answer.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
//	encoding/base64, encoding/json, testing.
//
// @reason    G5 ports one provider adapter at a time. Inworld is the Basic-auth
//
//	case, so these rows pin that the credential reaches the declared
//	header and that a 200 without `audioContent` is a failure rather
//	than an empty file.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_InworldSpeech pins the request body, the Basic header,
// and the base64 answer.
func TestMediaCallService_InworldSpeech(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	router.credential = provider.StaticKey("ep-inworld", "key-iw", "dXNlcjpwYXNz")
	caller.answer = dataplane.MediaResponse{
		Status: 200, Body: []byte(`{"audioContent":"` + base64.StdEncoding.EncodeToString([]byte("mp3-bytes")) + `"}`),
	}

	req := schema.SpeechRequest{Model: "inworld/inworld-tts-1.5-max", Input: "hello", Voice: "Alexa"}
	answer, call, err := svc.Speech(context.Background(), req, nil, "gky_iw")
	if err != nil {
		t.Fatalf("Speech() error = %v", err)
	}
	if string(answer.Body) != "mp3-bytes" {
		t.Fatalf("answer = %q, want the decoded audio", answer.Body)
	}
	if got := caller.requests[0].Headers["Authorization"]; got != "Basic dXNlcjpwYXNz" {
		t.Fatalf("authorization = %q, want the Basic credential", got)
	}
	var body inworldSpeechBody
	if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
		t.Fatalf("decoding Inworld body: %v", err)
	}
	if body.Text != "hello" || body.VoiceID != "Alexa" || body.ModelID != "inworld-tts-1.5-max" {
		t.Fatalf("body = %+v, want text/voice/model filled", body)
	}
	if body.AudioConfig.AudioEncoding != "MP3" {
		t.Fatalf("audio encoding = %q, want MP3", body.AudioConfig.AudioEncoding)
	}
	if got := call.SpeechOutputFormat(req); got != "mp3" {
		t.Fatalf("output format = %q, want mp3", got)
	}
}

// TestMediaCallService_InworldDefaultVoiceAndRefusal covers the provider's voice
// default and the 200-without-audio refusal.
func TestMediaCallService_InworldDefaultVoiceAndRefusal(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"usage":{}}`)}

	_, _, err := svc.Speech(context.Background(), schema.SpeechRequest{
		Model: "inworld/inworld-tts-1.5-mini", Input: "hello",
	}, nil, "gky_iw")
	if err == nil {
		t.Fatal("Speech() error = nil, want a 200 without audio refused")
	}
	var body inworldSpeechBody
	if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
		t.Fatalf("decoding Inworld body: %v", err)
	}
	if body.VoiceID != inworldDefaultVoice {
		t.Fatalf("voice = %q, want the provider default %q", body.VoiceID, inworldDefaultVoice)
	}
	if router.successes != 0 {
		t.Fatalf("successes = %d, want an empty answer recorded as a failure", router.successes)
	}
}
