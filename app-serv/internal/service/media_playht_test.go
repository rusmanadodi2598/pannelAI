// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_playht_test.go
// @for       The PlayHT speech adapter: body, the paired credential headers, and
//
//	the declared Accept header.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
//	encoding/json, testing.
//
// @reason    G5 ports one provider adapter at a time. PlayHT is the two-header
//
//	credential case (`userId:apiKey`), so these rows pin the split, the
//	voice default, and that the OpenAI-shaped control still gets one
//	Authorization header.
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

// TestMediaCallService_PlayHTSpeech pins the request body and both credential
// headers the kind's block declares.
func TestMediaCallService_PlayHTSpeech(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	router.credential = provider.StaticKey("ep-playht", "key-ph", "user_7:ph_secret")
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("mp3-bytes")}

	req := schema.SpeechRequest{Model: "playht/Play3.0-mini", Input: "hello", Voice: "s3://voice/one"}
	answer, call, err := svc.Speech(context.Background(), req, nil, "gky_ph")
	if err != nil {
		t.Fatalf("Speech() error = %v", err)
	}
	if string(answer.Body) != "mp3-bytes" {
		t.Fatalf("answer = %q, want the upstream bytes", answer.Body)
	}
	headers := caller.requests[0].Headers
	if headers["X-USER-ID"] != "user_7" || headers["Authorization"] != "Bearer ph_secret" {
		t.Fatalf("credential headers = %v, want the split pair", headers)
	}
	if headers["Accept"] != "audio/mpeg" {
		t.Fatalf("accept = %q, want audio/mpeg", headers["Accept"])
	}
	var body playhtSpeechBody
	if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
		t.Fatalf("decoding PlayHT body: %v", err)
	}
	if body.Text != "hello" || body.Voice != "s3://voice/one" || body.VoiceEngine != "Play3.0-mini" {
		t.Fatalf("body = %+v, want text/voice/engine filled", body)
	}
	if body.OutputFormat != "mp3" || body.Speed != 1 {
		t.Fatalf("body = %+v, want the reference's mp3/speed shape", body)
	}
	if got := call.SpeechOutputFormat(req); got != "mp3" {
		t.Fatalf("output format = %q, want mp3", got)
	}
}

// TestMediaCallService_PlayHTDefaultVoice pins the reference's default voice for
// a caller that names none.
func TestMediaCallService_PlayHTDefaultVoice(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("mp3-bytes")}

	if _, _, err := svc.Speech(context.Background(), schema.SpeechRequest{
		Model: "playht/PlayDialog", Input: "hello",
	}, nil, "gky_ph"); err != nil {
		t.Fatalf("Speech() error = %v", err)
	}
	var body playhtSpeechBody
	if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
		t.Fatalf("decoding PlayHT body: %v", err)
	}
	if body.Voice != playHTDefaultVoice {
		t.Fatalf("voice = %q, want the reference default", body.Voice)
	}
}
