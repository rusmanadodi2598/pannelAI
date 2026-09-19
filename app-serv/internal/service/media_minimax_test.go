// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_minimax_test.go
// @for       The MiniMax speech adapter: body defaults, hex audio, and the
//
//	business failure its 200 envelope can carry.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
//	encoding/hex, encoding/json, testing.
//
// @reason    G5 ports one provider adapter at a time. MiniMax is the first whose
//
//	200 can be a failure (`base_resp.status_code`), so these rows pin
//	that the call is recorded as a failure the client saw, not as a
//	served call, and that the shared adapter serves both the .io and the
//	.cn provider.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_MinimaxSpeech pins the request body and the hex answer,
// including the provider's own voice default.
func TestMediaCallService_MinimaxSpeech(t *testing.T) {
	cases := []struct {
		name      string
		model     string
		voice     string
		wantVoice string
	}{
		{name: "voice omitted", model: "minimax/speech-2.8-hd", wantVoice: minimaxDefaultVoice},
		{name: "voice supplied", model: "minimax-cn/speech-2.6-turbo", voice: "male-qn-qingse", wantVoice: "male-qn-qingse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			router.credential = provider.StaticKey("ep-minimax", "key-mm", "mm-secret")
			caller.answer = dataplane.MediaResponse{
				Status: 200, Body: []byte(`{"data":{"audio":"` + hex.EncodeToString([]byte("mp3-bytes")) + `"},"extra_info":{"audio_format":"mp3"},"base_resp":{"status_code":0}}`),
			}

			req := schema.SpeechRequest{Model: tc.model, Input: "hello", Voice: tc.voice}
			answer, call, err := svc.Speech(context.Background(), req, nil, "gky_mm")
			if err != nil {
				t.Fatalf("Speech() error = %v", err)
			}
			if string(answer.Body) != "mp3-bytes" {
				t.Fatalf("answer = %q, want the decoded hex audio", answer.Body)
			}
			var body minimaxSpeechBody
			if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
				t.Fatalf("decoding MiniMax body: %v", err)
			}
			if body.Model != call.UpstreamModel || body.Text != "hello" {
				t.Fatalf("body = %+v, want model=%q text=hello", body, call.UpstreamModel)
			}
			if body.VoiceSetting.VoiceID != tc.wantVoice {
				t.Fatalf("voice = %q, want %q", body.VoiceSetting.VoiceID, tc.wantVoice)
			}
			if body.Stream || body.OutputFormat != "hex" || body.LanguageBoost != "auto" {
				t.Fatalf("body = %+v, want the reference's non-streaming hex shape", body)
			}
			if body.AudioSetting.Format != "mp3" || body.AudioSetting.SampleRate != minimaxSampleRate {
				t.Fatalf("audio setting = %+v, want the declared mp3 output", body.AudioSetting)
			}
			if got := call.SpeechOutputFormat(req); got != "mp3" {
				t.Fatalf("output format = %q, want mp3", got)
			}
			if router.successes != 1 {
				t.Fatalf("successes = %d, want the served call recorded once", router.successes)
			}
		})
	}
}

// TestMediaCallService_MinimaxBusinessFailure pins the envelope rule: a 200 whose
// `base_resp.status_code` is non-zero is a failure the client sees, so the call
// is recorded as one instead of appearing in Usage as a served request.
func TestMediaCallService_MinimaxBusinessFailure(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{
		Status: 200, Body: []byte(`{"base_resp":{"status_code":1004,"status_msg":"invalid voice"}}`),
	}

	_, _, err := svc.Speech(context.Background(), schema.SpeechRequest{
		Model: "minimax/speech-2.8-hd", Input: "hello",
	}, nil, "gky_mm")
	if err == nil {
		t.Fatal("Speech() error = nil, want the provider's business failure surfaced")
	}
	failure := dataplane.AsError(err)
	if failure.Code != dataplane.CodeUpstreamError {
		t.Fatalf("code = %s, want %s", failure.Code, dataplane.CodeUpstreamError)
	}
	if router.successes != 0 || len(router.failures) != 1 {
		t.Fatalf("health = %d successes / %v, want the call recorded as a failure",
			router.successes, router.failures)
	}
}
