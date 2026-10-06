// Handlers serve the HTTP surface of app-serv.
//
// @file      internal/handler/dataplane_error_log_test.go
// @for       The request id a data-plane error line has to carry.
// @uses      bytes, encoding/json, log/slog, net/http, net/http/httptest, testing
// @reason    A router access-log line holds the request id and a handler's own failure line held none, so an incident could not be joined from the two records the way AGENTS.md §1.6 requires. This pins the seam that makes the handler side carry it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-10-04
package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
)

// traceRecordingWriter is a ResponseWriter that knows its request id and fails
// every body write, which is what forces the error path under test.
type traceRecordingWriter struct {
	header http.Header
	id     string
}

func (w *traceRecordingWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *traceRecordingWriter) WriteHeader(int) {}

func (w *traceRecordingWriter) Write([]byte) (int, error) {
	return 0, errors.New("the connection closed mid-write")
}

func (w *traceRecordingWriter) RequestID() string { return w.id }

func TestWriteDataPlaneErrorLogsTheRequestID(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	writer := &traceRecordingWriter{id: "trace-abc"}
	writeDataPlaneError(writer, dataplane.InternalError("the upstream failed", nil))

	var line map[string]any
	if err := json.Unmarshal(logged.Bytes(), &line); err != nil {
		t.Fatalf("the log output is not one JSON line: %v (%q)", err, logged.String())
	}
	if got := line["request_id"]; got != "trace-abc" {
		t.Fatalf("request_id = %v, want the id the writer carries", got)
	}
}

// failingWriter answers every ResponseWriter question but reports the body write
// as failed, and deliberately carries no request id: that is the shape a writer
// from a middleware-less server or a bare test recorder has.
type failingWriter struct {
	*httptest.ResponseRecorder
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("the connection closed mid-write")
}

func TestRequestIDOfToleratesAWriterWithoutOne(t *testing.T) {
	// The failure still has to be logged, with an empty id rather than a panic or
	// a dropped line.
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	writeDataPlaneError(failingWriter{httptest.NewRecorder()}, dataplane.InternalError("the upstream failed", nil))
	if logged.Len() == 0 {
		t.Fatal("nothing was logged, want a line whose request id is simply absent")
	}
	var line map[string]any
	if err := json.Unmarshal(logged.Bytes(), &line); err != nil {
		t.Fatalf("the log output is not one JSON line: %v", err)
	}
	if got := line["request_id"]; got != "" {
		t.Fatalf("request_id = %v, want the empty string for a writer that carries none", got)
	}
}
