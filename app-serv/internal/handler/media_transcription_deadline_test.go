// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/media_transcription_deadline_test.go
// @for       That a slow 25 MB transcription upload outlives the server's short ReadTimeout.
// @uses      io, net/http, net/http/httptest, testing, time.
// @reason    The server reads JSON bodies within 30 seconds, but a lawful transcription upload can take minutes on a slow link. The end-to-end form is what proves the handler renews the connection deadline instead of letting the server cut the upload mid-body.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-22
package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMediaHandler_TranscribeOutlivesTheServerReadTimeout sends the multipart
// body in two writes with a pause longer than the server's ReadTimeout. Without
// the route's deadline renewal the second write would find a closed connection.
func TestMediaHandler_TranscribeOutlivesTheServerReadTimeout(t *testing.T) {
	f, _ := newMediaHandlerFixture(t)
	fixture := transcriptionRequest(t, "/api/v1/audio/transcriptions", "openai/whisper-1", "clip.mp3", []byte("audio-bytes"))
	body, err := io.ReadAll(fixture.Body)
	if err != nil {
		t.Fatalf("reading the multipart body: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(f.Transcribe))
	server.Config.ReadTimeout = 500 * time.Millisecond
	server.Start()
	defer server.Close()

	reader, writer := io.Pipe()
	go func() {
		defer func() { _ = writer.Close() }()
		half := len(body) / 2
		if _, err := writer.Write(body[:half]); err != nil {
			return
		}
		time.Sleep(900 * time.Millisecond)
		_, _ = writer.Write(body[half:])
	}()

	request, err := http.NewRequest(http.MethodPost, server.URL, reader)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	request.Header.Set("Content-Type", fixture.Header.Get("Content-Type"))
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("uploading through the server: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		answer, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, want 200 (body: %s)", response.StatusCode, answer)
	}
}
