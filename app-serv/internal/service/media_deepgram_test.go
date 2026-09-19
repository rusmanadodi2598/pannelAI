// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_deepgram_test.go
// @for       The Deepgram STT adapter: binary request shape, auth, query, MIME,
//
//	and nested transcript normalization.
//
// @uses      internal/dataplane, internal/provider, internal/schema, context,
//
//	encoding/json, net/url, strings, testing.
//
// @reason    G5 ports one provider adapter at a time. Deepgram is the smallest
//
//	decision-free adapter: one binary POST, no polling, and a documented
//	`Authorization: Token` header. These table cases prove its request
//	shape without weakening the shared selection, health, or accounting
//	pipeline.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_DeepgramTranscribe pins the adapter's complete request
// and answer contract for language and auto-detection paths.
func TestMediaCallService_DeepgramTranscribe(t *testing.T) {
	cases := []struct {
		name          string
		form          schema.TranscriptionForm
		wantType      string
		wantModel     string
		wantLanguage  string
		wantDetection string
	}{
		{
			name: "explicit language and MIME",
			form: schema.TranscriptionForm{
				Model: "deepgram/nova-3", Language: "id", Filename: "clip.mp3",
				ContentType: "audio/mpeg", File: []byte("audio-bytes"),
			},
			wantType: "audio/mpeg", wantModel: "nova-3", wantLanguage: "id",
		},
		{
			name: "language detection and extension fallback",
			form: schema.TranscriptionForm{
				Model: "dg/nova-2", Filename: "clip.wav",
				ContentType: "application/octet-stream", File: []byte("wav-bytes"),
			},
			wantType: "audio/wav", wantModel: "nova-2", wantDetection: "true",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, router := mediaCallFixture(t)
			router.credential = provider.StaticKey("ep-deepgram", "key-deepgram", "dg-secret")
			caller.answer = dataplane.MediaResponse{
				Status: 200,
				Body:   []byte(`{"results":{"channels":[{"alternatives":[{"transcript":"hello from deepgram"}]}]}}`),
			}

			answer, call, err := svc.Transcribe(context.Background(), tc.form, nil, "gky_deepgram")
			if err != nil {
				t.Fatalf("Transcribe() error = %v", err)
			}
			if call.ProviderID != "deepgram" || call.UpstreamModel != tc.wantModel {
				t.Fatalf("call = %s/%s, want deepgram/%s", call.ProviderID, call.UpstreamModel, tc.wantModel)
			}
			if string(answer.Body) != `{"text":"hello from deepgram"}` {
				t.Fatalf("answer = %s, want normalized transcript", answer.Body)
			}

			request := caller.requests[0]
			if request.Method != "POST" || !bytes.Equal(request.Body, tc.form.File) {
				t.Fatalf("request = %s/%q, want POST with raw audio bytes", request.Method, request.Body)
			}
			if request.Headers["Content-Type"] != tc.wantType {
				t.Fatalf("content type = %q, want %q", request.Headers["Content-Type"], tc.wantType)
			}
			if request.Headers["Authorization"] != "Token dg-secret" {
				t.Fatalf("authorization = %q, want Token auth", request.Headers["Authorization"])
			}
			parsed, err := url.Parse(request.URL)
			if err != nil {
				t.Fatalf("parsing target: %v", err)
			}
			query := parsed.Query()
			if query.Get("model") != tc.wantModel || query.Get("smart_format") != "true" || query.Get("punctuate") != "true" {
				t.Fatalf("query = %v, want model + smart_format + punctuate", query)
			}
			if query.Get("language") != tc.wantLanguage || query.Get("detect_language") != tc.wantDetection {
				t.Fatalf("language query = %v, want language=%q detect_language=%q", query, tc.wantLanguage, tc.wantDetection)
			}
		})
	}
}

// TestTranscriptionContentTypeUsesSafeAudioValues is the MIME boundary table:
// parsed audio values and known extensions are allowed, while non-audio and
// unknown values fall back without copying arbitrary header text outbound.
func TestTranscriptionContentTypeUsesSafeAudioValues(t *testing.T) {
	cases := []struct {
		name string
		form schema.TranscriptionForm
		want string
	}{
		{name: "audio MIME", form: schema.TranscriptionForm{ContentType: "audio/webm; codecs=opus", Filename: "upload.bin"}, want: "audio/webm"},
		{name: "known extension", form: schema.TranscriptionForm{ContentType: "", Filename: "upload.flac"}, want: "audio/flac"},
		{name: "non-audio MIME uses extension", form: schema.TranscriptionForm{ContentType: "text/html", Filename: "upload.wav"}, want: "audio/wav"},
		{name: "unknown values use octet stream", form: schema.TranscriptionForm{ContentType: "application/x-custom", Filename: "upload.bin"}, want: "application/octet-stream"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transcriptionContentType(tc.form); got != tc.want {
				t.Fatalf("content type = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTranscriptionReader covers the nested answer, an empty but valid
// answer, malformed JSON, and the benign non-Deepgram control that reads
// nothing because the upstream answer already carries the OpenAI shape.
func TestTranscriptionReader(t *testing.T) {
	cases := []struct {
		name    string
		format  string
		body    []byte
		want    string
		wantErr bool
		wantNil bool
	}{
		{
			name: "nested transcript", format: "deepgram",
			body: []byte(`{"results":{"channels":[{"alternatives":[{"transcript":"hello"}]}]}}`),
			want: `{"text":"hello"}`,
		},
		{
			name: "empty channels", format: "deepgram",
			body: []byte(`{"results":{"channels":[]}}`), want: `{"text":""}`,
		},
		{
			name: "malformed JSON", format: "deepgram", body: []byte(`not-json`), wantErr: true,
		},
		{
			name: "other format reads nothing", format: "openai", wantNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			read := transcriptionReader(registryMedia(tc.format))
			if tc.wantNil {
				if read != nil {
					t.Fatal("transcriptionReader() = a reader, want none for a format that needs no reading")
				}
				return
			}
			got, err := read(200, tc.body)
			if (err != nil) != tc.wantErr {
				t.Fatalf("reader error = %v, want error = %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if string(got) != tc.want {
				t.Fatalf("body = %s, want %s", got, tc.want)
			}
			var decoded struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(got, &decoded); err != nil {
				t.Fatalf("normalized body = %s, want valid JSON text: %v", got, err)
			}
		})
	}
}

func registryMedia(format string) registry.MediaConfig { return registry.MediaConfig{Format: format} }
