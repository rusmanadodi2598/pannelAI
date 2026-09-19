// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_transcription_test.go
// @for       The upload bound and the upload-name reduction that keep a
//
//	transcription from steering an upstream's storage or exhausting the
//	gateway (SPEC-API-001 §7.10, OWASP A01/A10).
//
// @uses      bytes, errors, mime/multipart, net/http, net/http/httptest,
//
//	strings, testing.
//
// @reason    A mitigation must hold for the whole class, not one payload
//
//	(OWASP §2.2/§2.4), so each table carries the benign inputs that must
//	survive alongside the shapes that must be refused, and the
//	trailing-separator case the first implementation missed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSafeUploadName pins the reduction: every input becomes one separator-free
// path segment, the benign names survive unchanged, and the degenerate shapes
// collapse to the default rather than reaching an upstream.
func TestSafeUploadName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "a benign name is kept", input: "clip.mp3", want: "clip.mp3"},
		{name: "a benign name with spaces is kept", input: "my voice.wav", want: "my voice.wav"},
		{name: "a relative traversal keeps only the file", input: "../../etc/passwd", want: "passwd"},
		{name: "a windows traversal keeps only the file", input: `..\..\windows\evil.dll`, want: "evil.dll"},
		{name: "an absolute path keeps only the file", input: "/var/audio/voice.wav", want: "voice.wav"},
		{name: "a trailing separator is trimmed, not forwarded", input: "../../", want: "audio"},
		{name: "a directory name is trimmed to its segment", input: "clips/", want: "clips"},
		{name: "a bare parent is refused", input: "..", want: "audio"},
		{name: "a bare dot is refused", input: ".", want: "audio"},
		{name: "an empty name is refused", input: "", want: "audio"},
		{name: "a whitespace name is refused", input: "   ", want: "audio"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := &multipart.FileHeader{Filename: tc.input}
			if got := safeUploadName(header); got != tc.want {
				t.Fatalf("safeUploadName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// failingReader stands in for an upload whose read fails part way, which is
// what a dropped connection looks like to readBounded.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

// TestReadBounded pins the upload bound: the benign sizes at and below the
// limit survive, one byte past it is refused rather than truncated, and the
// degenerate bodies are named.
func TestReadBounded(t *testing.T) {
	cases := []struct {
		name    string
		reader  func() *bytes.Reader
		wantErr string
	}{
		{name: "a small upload is read whole", reader: func() *bytes.Reader { return bytes.NewReader([]byte("audio")) }},
		{name: "an upload exactly at the limit is allowed", reader: func() *bytes.Reader {
			return bytes.NewReader(bytes.Repeat([]byte("a"), 16))
		}},
		{name: "one byte past the limit is refused", reader: func() *bytes.Reader {
			return bytes.NewReader(bytes.Repeat([]byte("a"), 17))
		}, wantErr: "too large"},
		{name: "an empty upload is refused", reader: func() *bytes.Reader { return bytes.NewReader(nil) }, wantErr: "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := readBounded(tc.reader(), 16)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("readBounded() error = %v, want it to name %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("readBounded() error = %v", err)
			}
			if len(content) == 0 {
				t.Fatal("readBounded() returned nothing for an allowed upload")
			}
		})
	}

	t.Run("a failed read is refused", func(t *testing.T) {
		if _, err := readBounded(failingReader{}, 16); err == nil {
			t.Fatal("readBounded() accepted a read that failed")
		}
	})
}

// TestReadTranscriptionForm_SanitizesTheFilename pins that the reduction is
// wired into the form, not merely available: the filename a caller crafts is
// the one that reaches the upstream part.
func TestReadTranscriptionForm_SanitizesTheFilename(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{name: "a benign name survives", filename: "clip.mp3", want: "clip.mp3"},
		{name: "a traversal reduces to its base", filename: "../../etc/passwd", want: "passwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form, err := ReadTranscriptionForm(transcriptionUpload(t, tc.filename))
			if err != nil {
				t.Fatalf("ReadTranscriptionForm() error = %v", err)
			}
			if form.Filename != tc.want {
				t.Fatalf("filename = %q, want %q", form.Filename, tc.want)
			}
		})
	}
}

// transcriptionUpload builds the multipart request ReadTranscriptionForm reads.
func transcriptionUpload(t *testing.T, filename string) *http.Request {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	if err := writer.WriteField("model", "openai/whisper-1"); err != nil {
		t.Fatalf("writing the model field: %v", err)
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("creating the file part: %v", err)
	}
	if _, err := part.Write([]byte("audio")); err != nil {
		t.Fatalf("writing the file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/audio/transcriptions", buffer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
