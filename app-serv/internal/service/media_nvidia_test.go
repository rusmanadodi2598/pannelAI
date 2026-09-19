// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_nvidia_test.go
// @for       The NVIDIA NIM TTS adapter: nested JSON request, voice default,
//
//	WAV output label, and the OpenAI-shaped control.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
//	encoding/json, testing.
//
// @reason    G5 ports one provider adapter at a time. NVIDIA is a small generic
//
//	TTS adapter with a declared model set and no polling or provider-specific
//	credential composition. These table cases pin its wire shape and keep
//	the existing OpenAI speech path from drifting.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_NvidiaSpeech pins the provider's nested request and the
// provider default voice, while the caller's explicit voice remains intact.
func TestMediaCallService_NvidiaSpeech(t *testing.T) {
	cases := []struct {
		name      string
		request   schema.SpeechRequest
		wantVoice string
		wantModel string
	}{
		{
			name:      "provider default voice",
			request:   schema.SpeechRequest{Model: "nvidia/fastpitch", Input: "hello"},
			wantVoice: "default", wantModel: "fastpitch",
		},
		{
			name:      "explicit voice",
			request:   schema.SpeechRequest{Model: "nvidia/tacotron2", Input: "hello", Voice: "female"},
			wantVoice: "female", wantModel: "tacotron2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			router.credential = provider.StaticKey("ep-nvidia", "key-nvidia", "nv-secret")
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("wav-bytes")}

			answer, call, err := svc.Speech(context.Background(), tc.request, nil, "gky_nvidia")
			if err != nil {
				t.Fatalf("Speech() error = %v", err)
			}
			if call.ProviderID != "nvidia" || call.UpstreamModel != tc.wantModel {
				t.Fatalf("call = %s/%s, want nvidia/%s", call.ProviderID, call.UpstreamModel, tc.wantModel)
			}
			if string(answer.Body) != "wav-bytes" {
				t.Fatalf("answer = %q, want upstream WAV bytes", answer.Body)
			}
			var body nvidiaSpeechBody
			if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
				t.Fatalf("decoding NVIDIA body: %v", err)
			}
			if body.Input.Text != tc.request.Input || body.Voice != tc.wantVoice || body.Model != tc.wantModel {
				t.Fatalf("body = %+v, want text=%q voice=%q model=%q", body, tc.request.Input, tc.wantVoice, tc.wantModel)
			}
			if caller.requests[0].Headers["Content-Type"] != "application/json" {
				t.Fatalf("content type = %q, want application/json", caller.requests[0].Headers["Content-Type"])
			}
			if caller.requests[0].Headers["Authorization"] != "Bearer nv-secret" {
				t.Fatalf("authorization = %q, want bearer credential", caller.requests[0].Headers["Authorization"])
			}
			if got := call.SpeechOutputFormat(tc.request); got != "wav" {
				t.Fatalf("output format = %q, want wav", got)
			}
		})
	}
}

// TestSpeechOutputFormatKeepsOpenAIControl is the benign control: a generic
// OpenAI-shaped call still reports the client's requested format/default.
func TestSpeechOutputFormatKeepsOpenAIControl(t *testing.T) {
	cases := []struct {
		name   string
		format string
		want   string
	}{
		{name: "the explicit opus format", format: "opus", want: "opus"},
		{name: "the default format", want: "mp3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := MediaCall{}
			if got := call.SpeechOutputFormat(schema.SpeechRequest{ResponseFormat: tc.format}); got != tc.want {
				t.Fatalf("format = %q, want %q", got, tc.want)
			}
		})
	}
}
