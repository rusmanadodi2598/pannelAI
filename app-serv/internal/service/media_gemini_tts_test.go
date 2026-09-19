// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_gemini_tts_test.go
// @for       The Gemini speech adapter: the generateContent URL, the prompt, the
//
//	voice, and the PCM-to-WAV answer.
//
// @uses      internal/dataplane, internal/provider, internal/schema, bytes, context,
//
//	encoding/base64, encoding/binary, encoding/json, testing.
//
// @reason    G5 ports one provider adapter at a time. Gemini is the case where
//
//	the model is a path segment, the credential a query parameter, and
//	the answer raw PCM, so these rows pin all three and the no-audio
//	refusal that a 200 can carry.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_GeminiSpeech pins the URL, the prompt forms, the voice
// default, and the WAV wrapping of the answer.
func TestMediaCallService_GeminiSpeech(t *testing.T) {
	cases := []struct {
		name      string
		request   schema.SpeechRequest
		wantText  string
		wantVoice string
	}{
		{
			name: "a plain prompt and the default voice",
			request: schema.SpeechRequest{
				Model: "gemini/gemini-2.5-flash-preview-tts", Input: "hello",
			},
			wantText: "Say: hello", wantVoice: geminiDefaultVoice,
		},
		{
			name: "a language and an explicit voice",
			request: schema.SpeechRequest{
				Model: "gemini/gemini-2.5-pro-preview-tts", Input: "halo", Language: "Indonesian", Voice: "Puck",
			},
			wantText: "Say in Indonesian: halo", wantVoice: "Puck",
		},
		{
			name: "a caller-written directive is kept",
			request: schema.SpeechRequest{
				Model: "gemini/gemini-2.5-flash-preview-tts", Input: "Read slowly: hello",
			},
			wantText: "Read slowly: hello", wantVoice: geminiDefaultVoice,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			router.credential = provider.StaticKey("ep-gemini", "key-gm", "gm-secret")
			pcm := []byte{0x01, 0x02, 0x03, 0x04}
			caller.answer = dataplane.MediaResponse{
				Status: 200,
				Body: []byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"data":"` +
					base64.StdEncoding.EncodeToString(pcm) + `"}}]}}]}`),
			}

			answer, call, err := svc.Speech(context.Background(), tc.request, nil, "gky_gm")
			if err != nil {
				t.Fatalf("Speech() error = %v", err)
			}
			request := caller.requests[0]
			wantURL := "https://generativelanguage.googleapis.com/v1beta/models/" +
				call.UpstreamModel + ":generateContent?key=gm-secret"
			if request.URL != wantURL {
				t.Fatalf("URL = %q, want %q", request.URL, wantURL)
			}
			var body geminiSpeechBody
			if err := json.Unmarshal(request.Body, &body); err != nil {
				t.Fatalf("decoding Gemini body: %v", err)
			}
			if got := body.Contents[0].Parts[0].Text; got != tc.wantText {
				t.Fatalf("prompt = %q, want %q", got, tc.wantText)
			}
			voice := body.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName
			if voice != tc.wantVoice {
				t.Fatalf("voice = %q, want %q", voice, tc.wantVoice)
			}
			if len(body.GenerationConfig.ResponseModalities) != 1 ||
				body.GenerationConfig.ResponseModalities[0] != "AUDIO" {
				t.Fatalf("modalities = %v, want AUDIO", body.GenerationConfig.ResponseModalities)
			}
			if len(answer.Body) != 44+len(pcm) {
				t.Fatalf("answer length = %d, want the PCM plus a WAV header", len(answer.Body))
			}
			if string(answer.Body[0:4]) != "RIFF" || string(answer.Body[8:12]) != "WAVE" {
				t.Fatalf("answer header = %q, want a RIFF/WAVE container", answer.Body[0:12])
			}
			if got := binary.LittleEndian.Uint32(answer.Body[24:28]); got != geminiSampleRate {
				t.Fatalf("sample rate = %d, want %d", got, geminiSampleRate)
			}
			if !bytes.Equal(answer.Body[44:], pcm) {
				t.Fatalf("payload = %v, want the decoded PCM", answer.Body[44:])
			}
			if got := call.SpeechOutputFormat(tc.request); got != "wav" {
				t.Fatalf("output format = %q, want wav", got)
			}
		})
	}
}

// TestMediaCallService_GeminiNoAudio pins the refusal a 200 can carry: Gemini
// answers a finish reason instead of audio, and the reason travels with the
// failure rather than being dropped.
func TestMediaCallService_GeminiNoAudio(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{
		Status: 200, Body: []byte(`{"candidates":[{"finishReason":"SAFETY"}]}`),
	}

	_, _, err := svc.Speech(context.Background(), schema.SpeechRequest{
		Model: "gemini/gemini-2.5-flash-preview-tts", Input: "hello",
	}, nil, "gky_gm")
	if err == nil {
		t.Fatal("Speech() error = nil, want a 200 without audio refused")
	}
	if failure := dataplane.AsError(err); failure.Code != dataplane.CodeUpstreamError {
		t.Fatalf("code = %s, want %s", failure.Code, dataplane.CodeUpstreamError)
	}
	if router.successes != 0 {
		t.Fatalf("successes = %d, want the refusal recorded as a failure", router.successes)
	}
}
