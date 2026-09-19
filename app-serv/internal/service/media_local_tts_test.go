// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_local_tts_test.go
// @for       The two self-hosted speech adapters: Coqui's optional speaker and
//
//	Tortoise's default voice, both answered as WAV.
//
// @uses      internal/dataplane, internal/schema, context, encoding/json, testing.
// @reason    G5 ports one provider adapter at a time. These two run on the
//
//	operator's own machine and take no credential, so the rows pin that
//	no Authorization header is sent at all (the G16 rule) and that the
//	route reports WAV, which is what both answer.
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
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_LocalSpeech pins both self-hosted bodies and that neither
// sends a credential header.
func TestMediaCallService_LocalSpeech(t *testing.T) {
	cases := []struct {
		name        string
		model       string
		voice       string
		wantBody    string
		wantVoice   string
		wantSpeaker string
	}{
		{
			name: "coqui without a speaker", model: "coqui/tts_models/en/ljspeech/tacotron2-DDC",
			wantBody: "coqui",
		},
		{
			name: "coqui with a speaker", model: "coqui/tts_models/en/ljspeech/tacotron2-DDC",
			voice: "p225", wantBody: "coqui", wantSpeaker: "p225",
		},
		{
			name: "tortoise defaults its voice", model: "tortoise/tortoise-v2",
			wantBody: "tortoise", wantVoice: tortoiseDefaultVoice,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, _ := mediaCallFixture(t)
			caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("wav-bytes")}

			req := schema.SpeechRequest{Model: tc.model, Input: "hello", Voice: tc.voice}
			answer, call, err := svc.Speech(context.Background(), req, nil, "gky_local")
			if err != nil {
				t.Fatalf("Speech() error = %v", err)
			}
			if string(answer.Body) != "wav-bytes" {
				t.Fatalf("answer = %q, want the upstream bytes", answer.Body)
			}
			request := caller.requests[0]
			if _, set := request.Headers["Authorization"]; set {
				t.Fatalf("authorization = %q, want no credential header for a no_auth provider",
					request.Headers["Authorization"])
			}
			if tc.wantBody == "coqui" {
				var body coquiSpeechBody
				if err := json.Unmarshal(request.Body, &body); err != nil {
					t.Fatalf("decoding Coqui body: %v", err)
				}
				if body.Text != "hello" || body.SpeakerID != tc.wantSpeaker {
					t.Fatalf("body = %+v, want text/hello speaker=%q", body, tc.wantSpeaker)
				}
			} else {
				var body tortoiseSpeechBody
				if err := json.Unmarshal(request.Body, &body); err != nil {
					t.Fatalf("decoding Tortoise body: %v", err)
				}
				if body.Text != "hello" || body.Voice != tc.wantVoice {
					t.Fatalf("body = %+v, want text/hello voice=%q", body, tc.wantVoice)
				}
			}
			if got := call.SpeechOutputFormat(req); got != "wav" {
				t.Fatalf("output format = %q, want wav", got)
			}
		})
	}
}
