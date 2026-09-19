// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_test.go
// @for       HTTP tests for the §7.10 audio routes: speech, transcription,
//
//	and voices.
//
// @uses      bytes, encoding/base64, mime/multipart, net/http,
//
//	net/http/httptest, strings, testing.
//
// @reason    Two of these routes answer with something other than JSON, which
//
//	is the part worth pinning: a client that asked for audio must not
//	receive an error envelope, and one that asked for base64 must not
//	receive bytes. The generation and search routes live in
//	media_generation_test.go, the shared guards in media_guard_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// TestMediaHandler_SpeechBytes pins the default answer: the audio itself, with
// the content type the caller's format implies.
func TestMediaHandler_SpeechBytes(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("mp3-bytes")}

	rr := do(t, http.MethodPost, "/api/v1/audio/speech",
		`{"model":"openai/gpt-4o-mini-tts","input":"hello","response_format":"opus"}`, f.Speech)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "audio/opus" {
		t.Fatalf("content type = %q, want audio/opus", got)
	}
	if rr.Body.String() != "mp3-bytes" {
		t.Fatalf("body = %q, want the upstream bytes", rr.Body.String())
	}
}

// TestMediaHandler_SpeechJSON pins the `?response_format=json` answer: the same
// bytes, base64-encoded, in the shape the reference's handler returns.
func TestMediaHandler_SpeechJSON(t *testing.T) {
	f, caller := newMediaHandlerFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("mp3-bytes")}

	rr := do(t, http.MethodPost, "/api/v1/audio/speech?response_format=json",
		`{"model":"openai/gpt-4o-mini-tts","input":"hello"}`, f.Speech)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["audio"] != base64.StdEncoding.EncodeToString([]byte("mp3-bytes")) {
		t.Fatalf("audio = %v, want the base64 payload", body["audio"])
	}
	if body["format"] != "mp3" {
		t.Fatalf("format = %v, want the default mp3", body["format"])
	}
}

// TestMediaHandler_Transcribe pins the multipart upload and the forwarded
// answer, for both a JSON and a plain-text upstream response.
func TestMediaHandler_Transcribe(t *testing.T) {
	cases := []struct {
		name        string
		answer      dataplane.MediaResponse
		wantType    string
		wantContain string
	}{
		{
			name:        "a JSON answer",
			answer:      dataplane.MediaResponse{Status: 200, Body: []byte(`{"text":"halo"}`)},
			wantType:    "application/json",
			wantContain: `"text":"halo"`,
		},
		{
			name:        "a plain-text answer",
			answer:      dataplane.MediaResponse{Status: 200, Body: []byte("halo")},
			wantType:    "text/plain; charset=utf-8",
			wantContain: "halo",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, caller := newMediaHandlerFixture(t)
			caller.answer = tc.answer

			request := transcriptionRequest(t, "/api/v1/audio/transcriptions", "openai/whisper-1", "clip.mp3", []byte("audio-bytes"))
			rr := httptest.NewRecorder()
			f.Transcribe(rr, request)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
			}
			if got := rr.Header().Get("Content-Type"); got != tc.wantType {
				t.Fatalf("content type = %q, want %q", got, tc.wantType)
			}
			if !strings.Contains(rr.Body.String(), tc.wantContain) {
				t.Fatalf("body = %q, want it to contain %q", rr.Body.String(), tc.wantContain)
			}
			if !strings.HasPrefix(caller.requests[0].Headers["Content-Type"], "multipart/form-data") {
				t.Fatalf("upstream content type = %q, want multipart", caller.requests[0].Headers["Content-Type"])
			}
		})
	}
}

// TestMediaHandler_TranscribeRequiresAFile pins the multipart validation: a
// body without a file is the caller's mistake and is reported as one.
func TestMediaHandler_TranscribeRequiresAFile(t *testing.T) {
	f, _ := newMediaHandlerFixture(t)
	request := transcriptionRequest(t, "/api/v1/audio/transcriptions", "openai/whisper-1", "", nil)
	rr := httptest.NewRecorder()
	f.Transcribe(rr, request)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"VALIDATION_ERROR"`) {
		t.Fatalf("body = %s, want the VALIDATION_ERROR code", rr.Body.String())
	}
}

// TestMediaHandler_Voices pins the catalog answer and its required parameter.
func TestMediaHandler_Voices(t *testing.T) {
	f, _ := newMediaHandlerFixture(t)

	rr := do(t, http.MethodGet, "/api/v1/audio/voices?provider=openai", "", f.Voices)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["object"] != "list" {
		t.Fatalf("object = %v, want the list envelope", body["object"])
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data = %v, want the declared voice", body["data"])
	}

	rr = do(t, http.MethodGet, "/api/v1/audio/voices", "", f.Voices)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing provider status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
}

// transcriptionRequest builds the multipart upload the transcription route
// reads, with an empty filename standing for an absent file part.
func transcriptionRequest(t *testing.T, target, model, filename string, content []byte) *http.Request {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	if err := writer.WriteField("model", model); err != nil {
		t.Fatalf("writing the model field: %v", err)
	}
	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("creating the file part: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("writing the file part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, target, buffer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
