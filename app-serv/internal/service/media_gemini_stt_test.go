// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_gemini_stt_test.go
// @for       The Gemini transcription adapter: the generateContent URL, the
//
//	inline audio, the prompt forms, and the joined answer.
//
// @uses      internal/dataplane, internal/domain, internal/provider,
//
//	internal/schema, bytes, context, encoding/base64, encoding/json,
//	testing.
//
// @reason    G5 ports one provider adapter at a time. Gemini is the case where
//
//	the model is a path segment, the credential a query parameter, and
//	the audio inline in a JSON body, so these rows pin all three plus
//	the empty transcript that is a legitimate answer.
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
	"encoding/json"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_GeminiTranscribe pins the URL, the prompt forms, and the
// inline audio the endpoint reads.
func TestMediaCallService_GeminiTranscribe(t *testing.T) {
	cases := []struct {
		name       string
		form       schema.TranscriptionForm
		wantPrompt string
		wantMIME   string
	}{
		{
			name: "the default prompt and an extension-derived MIME",
			form: schema.TranscriptionForm{
				Model: "gemini/gemini-2.5-flash", Filename: "clip.mp3", File: []byte("audio-bytes"),
			},
			wantPrompt: geminiTranscriptionPrompt, wantMIME: "audio/mpeg",
		},
		{
			name: "a caller prompt, a language, and a parsed audio MIME",
			form: schema.TranscriptionForm{
				Model: "gemini/gemini-2.5-pro", Filename: "clip.bin",
				ContentType: "audio/wav; charset=binary", Language: "Indonesian",
				Prompt: "names", File: []byte("audio-bytes"),
			},
			wantPrompt: "names Language: Indonesian.", wantMIME: "audio/wav",
		},
		{
			name: "a non-audio MIME and an unknown suffix fall back",
			form: schema.TranscriptionForm{
				Model: "gemini/gemini-2.0-flash", Filename: "clip.bin",
				ContentType: "video/mp4", File: []byte("audio-bytes"),
			},
			wantPrompt: geminiTranscriptionPrompt, wantMIME: "application/octet-stream",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			router.credential = provider.StaticKey("ep-gemini", "key-gm", "gm-secret")
			caller.answer = dataplane.MediaResponse{
				Status: 200,
				Body:   []byte(`{"candidates":[{"content":{"parts":[{"text":"hello world"}]}}]}`),
			}

			answer, call, err := svc.Transcribe(context.Background(), tc.form, nil, "gky_gm")
			if err != nil {
				t.Fatalf("Transcribe() error = %v", err)
			}
			request := caller.requests[0]
			wantURL := "https://generativelanguage.googleapis.com/v1beta/models/" +
				call.UpstreamModel + ":generateContent?key=gm-secret"
			if request.URL != wantURL {
				t.Fatalf("URL = %q, want %q", request.URL, wantURL)
			}
			if got := request.Headers["Content-Type"]; got != "application/json" {
				t.Fatalf("content type = %q, want application/json", got)
			}
			var body geminiTranscriptionBody
			if err := json.Unmarshal(request.Body, &body); err != nil {
				t.Fatalf("decoding Gemini body: %v", err)
			}
			parts := body.Contents[0].Parts
			if len(parts) != 2 {
				t.Fatalf("parts = %d, want the instruction and the audio", len(parts))
			}
			if parts[0].Text != tc.wantPrompt {
				t.Fatalf("prompt = %q, want %q", parts[0].Text, tc.wantPrompt)
			}
			if parts[1].InlineData == nil || parts[1].InlineData.MIMEType != tc.wantMIME {
				t.Fatalf("inline audio = %+v, want MIME %q", parts[1].InlineData, tc.wantMIME)
			}
			decoded, err := base64.StdEncoding.DecodeString(parts[1].InlineData.Data)
			if err != nil || !bytes.Equal(decoded, tc.form.File) {
				t.Fatalf("inline data = %q, want the uploaded bytes", parts[1].InlineData.Data)
			}
			if string(answer.Body) != `{"text":"hello world"}` {
				t.Fatalf("answer = %s, want the normalized transcript", answer.Body)
			}
		})
	}
}

// TestMediaCallService_GeminiTranscribeJoinsParts pins the answer read: Gemini
// spreads a transcript over the candidate's parts, and the route returns one
// text field.
func TestMediaCallService_GeminiTranscribeJoinsParts(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	router.credential = provider.StaticKey("ep-gemini", "key-gm", "gm-secret")
	caller.answer = dataplane.MediaResponse{
		Status: 200,
		Body:   []byte(`{"candidates":[{"content":{"parts":[{"text":"hello"},{"text":""},{"text":" world"}]}}]}`),
	}

	answer, _, err := svc.Transcribe(context.Background(), schema.TranscriptionForm{
		Model: "gemini/gemini-2.5-flash", Filename: "clip.mp3", File: []byte("audio-bytes"),
	}, nil, "")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if string(answer.Body) != `{"text":"hello world"}` {
		t.Fatalf("answer = %s, want the parts joined", answer.Body)
	}
}

// TestMediaCallService_GeminiTranscribeEmptyAnswer pins that an answer without a
// candidate is an empty transcript, not a failure: silence transcribes to
// nothing, and the reference answers the same way.
func TestMediaCallService_GeminiTranscribeEmptyAnswer(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	router.credential = provider.StaticKey("ep-gemini", "key-gm", "gm-secret")
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"candidates":[]}`)}

	answer, _, err := svc.Transcribe(context.Background(), schema.TranscriptionForm{
		Model: "gemini/gemini-2.5-flash", Filename: "clip.mp3", File: []byte("audio-bytes"),
	}, nil, "")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if string(answer.Body) != `{"text":""}` {
		t.Fatalf("answer = %s, want an empty transcript", answer.Body)
	}
	if router.successes != 1 {
		t.Fatalf("successes = %d, want the call recorded as served", router.successes)
	}
}

// TestMediaCallService_GeminiTranscribeMalformedAnswer pins that an unreadable
// 200 is an internal error rather than a fabricated empty transcript.
func TestMediaCallService_GeminiTranscribeMalformedAnswer(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	router.credential = provider.StaticKey("ep-gemini", "key-gm", "gm-secret")
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`not json`)}

	_, _, err := svc.Transcribe(context.Background(), schema.TranscriptionForm{
		Model: "gemini/gemini-2.5-flash", Filename: "clip.mp3", File: []byte("audio-bytes"),
	}, nil, "")
	if err == nil {
		t.Fatal("Transcribe() accepted an unreadable answer")
	}
	if failure := dataplane.AsError(err); failure.Code != dataplane.CodeInternal {
		t.Fatalf("code = %s, want %s", failure.Code, dataplane.CodeInternal)
	}
	if router.successes != 0 {
		t.Fatalf("successes = %d, want the unreadable answer not recorded as served", router.successes)
	}
}

// TestMediaFormatSupported_GeminiStt pins the gate: each Gemini format is bound
// to its own kind, so a declaration under the other kind is still refused by
// name.
func TestMediaFormatSupported_GeminiStt(t *testing.T) {
	cases := []struct {
		format string
		kind   domain.MediaKind
		want   bool
	}{
		{format: "gemini-stt", kind: domain.MediaKindSTT, want: true},
		{format: "gemini-stt", kind: domain.MediaKindTTS, want: false},
		{format: "gemini-tts", kind: domain.MediaKindTTS, want: true},
		{format: "gemini-tts", kind: domain.MediaKindSTT, want: false},
		{format: "edge-tts", kind: domain.MediaKindTTS, want: false},
	}
	for _, tc := range cases {
		if got := mediaFormatSupported(tc.kind, tc.format); got != tc.want {
			t.Fatalf("mediaFormatSupported(%s, %s) = %v, want %v", tc.kind, tc.format, got, tc.want)
		}
	}
}
