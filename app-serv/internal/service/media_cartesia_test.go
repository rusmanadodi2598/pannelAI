// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_cartesia_test.go
// @for       The Cartesia speech adapter: request body, version/auth headers,
//
// voice omission, and MP3 output labeling.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
// encoding/json, testing.
//
// @reason    G5 ports one provider adapter at a time. Cartesia is a synchronous
//
// generic TTS adapter with one documented API-key header and a fixed MP3
// output shape. These table cases keep its provider wire separate from
// NVIDIA and the OpenAI-compatible control.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_CartesiaSpeech pins the Cartesia request for the voice
// omitted and explicit cases, including its fixed output format and version.
func TestMediaCallService_CartesiaSpeech(t *testing.T) {
	cases := []struct {
		name         string
		request      schema.SpeechRequest
		wantModel    string
		wantVoice    string
		wantVoiceSet bool
	}{
		{
			name: "voice omitted", request: schema.SpeechRequest{Model: "cartesia/sonic-2", Input: "hello"},
			wantModel: "sonic-2",
		},
		{
			name: "voice id supplied", request: schema.SpeechRequest{Model: "cartesia/sonic-3", Input: "hello", Voice: "voice-123"},
			wantModel: "sonic-3", wantVoice: "voice-123", wantVoiceSet: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			router.credential = provider.StaticKey("ep-cartesia", "key-cartesia", "ca-secret")
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("mp3-bytes")}

			answer, call, err := svc.Speech(context.Background(), tc.request, nil, "gky_cartesia")
			if err != nil {
				t.Fatalf("Speech() error = %v", err)
			}
			if call.ProviderID != "cartesia" || call.UpstreamModel != tc.wantModel {
				t.Fatalf("call = %s/%s, want cartesia/%s", call.ProviderID, call.UpstreamModel, tc.wantModel)
			}
			if string(answer.Body) != "mp3-bytes" {
				t.Fatalf("answer = %q, want MP3 bytes", answer.Body)
			}
			var body cartesiaSpeechBody
			if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
				t.Fatalf("decoding Cartesia body: %v", err)
			}
			if body.ModelID != tc.wantModel || body.Transcript != "hello" {
				t.Fatalf("body = %+v, want model=%q transcript=hello", body, tc.wantModel)
			}
			if (body.Voice != nil) != tc.wantVoiceSet {
				t.Fatalf("voice set = %v, want %v", body.Voice != nil, tc.wantVoiceSet)
			}
			if body.Voice != nil && (body.Voice.Mode != "id" || body.Voice.ID != tc.wantVoice) {
				t.Fatalf("voice = %+v, want id/%q", body.Voice, tc.wantVoice)
			}
			if body.OutputFormat.Container != "mp3" || body.OutputFormat.BitRate != 128000 || body.OutputFormat.SampleRate != 44100 {
				t.Fatalf("output_format = %+v, want mp3/128000/44100", body.OutputFormat)
			}
			request := caller.requests[0]
			if request.Headers["Content-Type"] != "application/json" {
				t.Fatalf("content type = %q, want application/json", request.Headers["Content-Type"])
			}
			if request.Headers["X-Api-Key"] != "ca-secret" {
				t.Fatalf("x-api-key = %q, want Cartesia credential", request.Headers["X-Api-Key"])
			}
			if request.Headers["Cartesia-Version"] != cartesiaVersion {
				t.Fatalf("Cartesia-Version = %q, want %q", request.Headers["Cartesia-Version"], cartesiaVersion)
			}
			if got := call.SpeechOutputFormat(tc.request); got != "mp3" {
				t.Fatalf("output format = %q, want mp3", got)
			}
		})
	}
}

// TestCartesiaSpeechOutputFormatIgnoresClientFormat is the safety control: the
// adapter requests and reports MP3 even when an OpenAI client asks for WAV,
// because Cartesia's fixed output_format is MP3.
func TestCartesiaSpeechOutputFormatIgnoresClientFormat(t *testing.T) {
	call := MediaCall{Media: mapMediaConfig("cartesia")}
	if got := call.SpeechOutputFormat(schema.SpeechRequest{ResponseFormat: "wav"}); got != "mp3" {
		t.Fatalf("format = %q, want mp3", got)
	}
}

func mapMediaConfig(format string) registry.MediaConfig { return registry.MediaConfig{Format: format} }
