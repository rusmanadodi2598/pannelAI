// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_audio_test.go
// @for       The speech, transcription, and voice payloads of §7.10.
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	encoding/json, mime/multipart, strings, testing.
//
// @reason    Each route's contract with its upstream is what these cases pin:
//
//	the speech payload the OpenAI shape expects, the multipart parts a
//	transcription must carry, and which providers can answer a voice
//	catalog at all.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_Speech pins the upstream payload: the model, the voice
// the caller or the model string names, and the optional fields.
func TestMediaCallService_Speech(t *testing.T) {
	cases := []struct {
		name      string
		req       schema.SpeechRequest
		wantVoice string
		wantModel string
		wantSpeed bool
	}{
		{
			name: "an explicit voice", req: schema.SpeechRequest{Model: "openai/gpt-4o-mini-tts", Input: "hello", Voice: "nova"},
			wantVoice: "nova", wantModel: "gpt-4o-mini-tts",
		},
		{
			// The reference's `provider/model/voice` shorthand is not ported:
			// a model id may carry a slash, so the segment would be read as a
			// model for one provider and a voice for another. The whole id is
			// kept and the default voice applies.
			name:      "a model id with a slash keeps its id",
			req:       schema.SpeechRequest{Model: "openrouter/openai/gpt-4o-mini-tts", Input: "hello"},
			wantVoice: schema.DefaultSpeechVoice, wantModel: "openai/gpt-4o-mini-tts",
		},
		{
			name: "the default voice", req: schema.SpeechRequest{Model: "openai/", Input: "hello"},
			wantVoice: schema.DefaultSpeechVoice, wantModel: "gpt-4o-mini-tts",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, _ := mediaCallFixture(t)
			_, _, err := svc.Speech(context.Background(), tc.req, nil, "")
			if err != nil {
				t.Fatalf("Speech() error = %v", err)
			}
			var body schema.SpeechBody
			if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
				t.Fatalf("decoding the upstream body: %v", err)
			}
			if body.Model != tc.wantModel || body.Voice != tc.wantVoice || body.Input != "hello" {
				t.Fatalf("body = %+v, want model %s and voice %s", body, tc.wantModel, tc.wantVoice)
			}
		})
	}
}

// TestMediaCallService_SpeechForwardsTheFormat pins that the caller's format
// reaches the upstream, which is what decides the audio the answer carries.
func TestMediaCallService_SpeechForwardsTheFormat(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	if _, _, err := svc.Speech(context.Background(), schema.SpeechRequest{
		Model: "openai/gpt-4o-mini-tts", Input: "hello", ResponseFormat: "opus",
	}, nil, ""); err != nil {
		t.Fatalf("Speech() error = %v", err)
	}
	var body schema.SpeechBody
	if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
		t.Fatalf("decoding the upstream body: %v", err)
	}
	if body.ResponseFormat != "opus" {
		t.Fatalf("response_format = %q, want the caller's opus", body.ResponseFormat)
	}
}

// TestMediaCallService_Transcribe pins the multipart body: the file part, the
// model, and the optional fields the reference forwards.
func TestMediaCallService_Transcribe(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	_, _, err := svc.Transcribe(context.Background(), schema.TranscriptionForm{
		Model: "openai/whisper-1", Language: "id", Prompt: "names", ResponseFormat: "json",
		Temperature: "0.2", Filename: "clip.mp3", File: []byte("audio-bytes"),
	}, nil, "")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	request := caller.requests[0]
	_, params, err := mime.ParseMediaType(request.Headers["Content-Type"])
	if err != nil {
		t.Fatalf("content type = %q, want multipart: %v", request.Headers["Content-Type"], err)
	}
	reader := multipart.NewReader(strings.NewReader(string(request.Body)), params["boundary"])
	fields := map[string]string{}
	var uploaded []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the multipart body: %v", err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading part %s: %v", part.FormName(), err)
		}
		if part.FormName() == "file" {
			uploaded = content
			if part.FileName() != "clip.mp3" {
				t.Fatalf("filename = %q, want clip.mp3", part.FileName())
			}
			continue
		}
		fields[part.FormName()] = string(content)
	}
	if string(uploaded) != "audio-bytes" {
		t.Fatalf("uploaded = %q, want the file's bytes", uploaded)
	}
	for name, want := range map[string]string{
		"model": "whisper-1", "language": "id", "prompt": "names",
		"response_format": "json", "temperature": "0.2",
	} {
		if fields[name] != want {
			t.Fatalf("field %s = %q, want %q", name, fields[name], want)
		}
	}
}

// TestMediaCallService_TranscribeOmitsEmptyFields pins that an absent optional
// field is not sent as an empty one: an upstream that validates `language`
// would reject an empty string where an omission is valid.
func TestMediaCallService_TranscribeOmitsEmptyFields(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	if _, _, err := svc.Transcribe(context.Background(), schema.TranscriptionForm{
		Model: "openai/whisper-1", Filename: "clip.mp3", File: []byte("audio-bytes"),
	}, nil, ""); err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if strings.Contains(string(caller.requests[0].Body), "language") {
		t.Fatalf("body = %s, want no language field", caller.requests[0].Body)
	}
}

// TestMediaCallService_TranscribeUpstreamRejection pins that a refused
// transcription is the client's error, with the upstream's message kept.
func TestMediaCallService_TranscribeUpstreamRejection(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 413, Body: []byte(`{"error":{"message":"file too large"}}`)}

	_, _, err := svc.Transcribe(context.Background(), schema.TranscriptionForm{
		Model: "openai/whisper-1", Filename: "clip.mp3", File: []byte("audio-bytes"),
	}, nil, "")
	if err == nil {
		t.Fatal("Transcribe() accepted a rejected upstream")
	}
	if !strings.Contains(err.Error(), "file too large") {
		t.Fatalf("error = %v, want the upstream's message", err)
	}
	if len(router.failures) != 1 {
		t.Fatalf("failures = %v, want the endpoint marked", router.failures)
	}
}

// TestMediaCallService_Voices pins which providers can answer a catalog.
func TestMediaCallService_Voices(t *testing.T) {
	cases := []struct {
		name          string
		provider      string
		wantVoices    int
		wantErrorText string
	}{
		{name: "a declared catalog", provider: "openai", wantVoices: 1},
		{name: "an unknown provider", provider: "ghost", wantErrorText: "is not in the registry"},
		{name: "a provider with no speech", provider: "brave-search", wantErrorText: "does not offer tts"},
		{name: "a provider that declares none", provider: "elevenlabs", wantErrorText: "declares no voice catalog"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := mediaCallFixture(t)
			voices, err := svc.Voices(tc.provider)
			if tc.wantErrorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrorText) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErrorText)
				}
				return
			}
			if err != nil {
				t.Fatalf("Voices() error = %v", err)
			}
			if len(voices) != tc.wantVoices {
				t.Fatalf("voices = %d, want %d", len(voices), tc.wantVoices)
			}
		})
	}
}
