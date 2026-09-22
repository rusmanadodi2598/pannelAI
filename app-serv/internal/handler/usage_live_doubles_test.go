// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_live_doubles_test.go
// @for       The double, the socket reader, and the route wrapper the live Usage
//
//	handler tests share.
//
// @uses      bufio, net, net/http, net/http/httptest, strings, sync, testing,
//
//	time, internal/domain, internal/repository, internal/schema, internal/service.
//
// @reason    A stream is the one answer whose timing is part of its contract
//
//	(SPEC-API-001 §4: the status line and SSE headers are committed on
//	the first frame, so a failure with no frame yet is still an ordinary
//	HTTP error). These tests drive a real socket rather than a recorder,
//	because httptest.ResponseRecorder reports a buffered answer as
//	healthy and cannot show that the first frame arrived while the
//	handler was still running.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// liveHandlerUsageDouble answers one page of records and counts the reads, so a
// test can tell "the stream read once per change" from "the stream polls".
type liveHandlerUsageDouble struct {
	mu      sync.Mutex
	records []domain.UsageRecord
	reads   int
}

func (d *liveHandlerUsageDouble) Record(context.Context, domain.UsageRecord) error { return nil }
func (d *liveHandlerUsageDouble) Summary(context.Context, domain.UsageFilter, domain.UsageGroupBy) (domain.UsageTotals, []domain.UsageGroupRow, error) {
	return domain.UsageTotals{}, nil, nil
}
func (d *liveHandlerUsageDouble) Timeseries(context.Context, domain.UsageFilter, domain.UsageGranularity) ([]domain.RateBucket, error) {
	return nil, nil
}
func (d *liveHandlerUsageDouble) List(context.Context, domain.UsageFilter, repository.PageQuery) ([]domain.UsageRecord, int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reads++
	return append([]domain.UsageRecord(nil), d.records...), int64(len(d.records)), nil
}
func (d *liveHandlerUsageDouble) GetByRequestID(context.Context, string) (domain.UsageRecord, error) {
	return domain.UsageRecord{}, domain.ErrUsageRecordNotFound
}
func (d *liveHandlerUsageDouble) MonthlyUsage(context.Context, string, time.Time) (domain.UsageTotals, error) {
	return domain.UsageTotals{}, nil
}

func (d *liveHandlerUsageDouble) readCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reads
}

// newLiveHandler builds the route's handler over the double, with the stream's
// cadences shortened so a test observes a keepalive and a change without waiting
// for the production intervals.
func newLiveHandler(t *testing.T, usage *liveHandlerUsageDouble) *UsageLiveHandler {
	t.Helper()
	svc, err := service.NewUsageLiveService(service.UsageLiveServiceDeps{Usage: usage})
	if err != nil {
		t.Fatalf("building the live service: %v", err)
	}
	handler := NewUsageLiveHandler(svc)
	handler.SetIntervals(50*time.Millisecond, 25*time.Millisecond)
	return handler
}

// liveRoute wraps the handler's Stream method the way the router registers it,
// so a test drives the same http.Handler the production table does.
func liveRoute(handler *UsageLiveHandler) http.Handler {
	return http.HandlerFunc(handler.Stream)
}

// liveStreamReader dials the handler over a real socket and reads frames one at
// a time, so an assertion can name the frame it read rather than a whole body.
type liveStreamReader struct {
	conn   net.Conn
	reader *bufio.Reader
}

func dialLiveStream(t *testing.T, handler http.Handler) *liveStreamReader {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatalf("dialing the stream: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("setting the socket deadline: %v", err)
	}
	// HTTP/1.0 keeps the body unframed: no chunked encoding, so a line read is a
	// line the handler wrote and the arrival timing is measurable.
	if _, err := conn.Write([]byte("GET /api/v1/usage/live HTTP/1.0\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatalf("writing the request: %v", err)
	}
	return &liveStreamReader{conn: conn, reader: bufio.NewReader(conn)}
}

// readHead reads the status line and headers.
func (r *liveStreamReader) readHead(t *testing.T) (string, http.Header) {
	t.Helper()
	statusLine, err := r.reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading the status line: %v", err)
	}
	headers := http.Header{}
	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading a header: %v", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
		name, value, found := strings.Cut(strings.TrimRight(line, "\r\n"), ":")
		if found {
			headers.Add(name, strings.TrimSpace(value))
		}
	}
	return statusLine, headers
}

// readFrame reads one complete SSE data frame and returns its payload.
func (r *liveStreamReader) readFrame(t *testing.T) string {
	t.Helper()
	var data []string
	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading a frame line: %v", err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if len(data) == 0 {
				continue
			}
			return strings.Join(data, "\n")
		}
		if payload, ok := strings.CutPrefix(trimmed, "data:"); ok {
			data = append(data, strings.TrimPrefix(payload, " "))
		}
	}
}
