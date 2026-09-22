// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/usage_live_test.go
// @for       The live Usage route's framing: the contract headers, the first
//
//	frame, and the keepalive.
//
// @uses      context, encoding/json, testing, time, internal/domain,
//
//	internal/schema.
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
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestUsageLiveHandler_StreamsTheContractHeaders pins the headers SPEC-API §4
// fixes for every stream, and the status line committed with the first frame.
func TestUsageLiveHandler_StreamsTheContractHeaders(t *testing.T) {
	usage := &liveHandlerUsageDouble{}
	stream := dialLiveStream(t, liveRoute(newLiveHandler(t, usage)))

	statusLine, headers := stream.readHead(t)
	if fields := strings.Fields(statusLine); len(fields) < 2 || fields[1] != "200" {
		t.Fatalf("status line = %q, want a 200", statusLine)
	}
	cases := []struct {
		header string
		want   string
		why    string
	}{
		{"Content-Type", "text/event-stream", "the client parses frames, not JSON"},
		{"Cache-Control", "no-cache", "a cached stream is not a stream"},
		{"X-Accel-Buffering", "no", "an nginx front end would otherwise buffer the whole answer"},
	}
	for _, tc := range cases {
		if got := headers.Get(tc.header); got != tc.want {
			t.Errorf("%s = %q, want %q: %s", tc.header, got, tc.want, tc.why)
		}
	}
}

// TestUsageLiveHandler_FirstFrameIsACompleteSnapshot pins the frame's contents
// on a fresh connection: the panel's first frame is full state, so a client that
// connected between changes still draws the right picture.
func TestUsageLiveHandler_FirstFrameIsACompleteSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	usage := &liveHandlerUsageDouble{}
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID: "req_1", TS: now, ProviderID: "openai", Model: "gpt-4o",
		TokensIn: 7, TokensOut: 3, CostUSD: "0.00100000", Status: domain.UsageStatusSuccess,
	}, "", now)
	if err != nil {
		t.Fatalf("building the record: %v", err)
	}
	usage.records = []domain.UsageRecord{record}

	stream := dialLiveStream(t, liveRoute(newLiveHandler(t, usage)))
	stream.readHead(t)
	payload := stream.readFrame(t)

	var frame schema.UsageLiveFrame
	if err := json.Unmarshal([]byte(payload), &frame); err != nil {
		t.Fatalf("decoding the first frame %s: %v", payload, err)
	}
	if frame.Active == nil {
		t.Error("active = null, want an array: the panel's first frame is full state")
	}
	if len(frame.Recent) != 1 {
		t.Fatalf("recent = %d entries, want 1", len(frame.Recent))
	}
	if frame.Recent[0].RequestID != "req_1" || frame.Recent[0].TokensIn != 7 {
		t.Fatalf("the first frame's recent entry = %+v, want the stored request", frame.Recent[0])
	}
	if frame.ErrorProvider != "" {
		t.Fatalf("error_provider = %q, want an empty string", frame.ErrorProvider)
	}
}

// TestUsageLiveHandler_KeepaliveIsAComment pins the keepalive's shape: an SSE
// comment, not a data frame, so a client that counts frames is not misled into
// thinking a change happened, and the connection survives an idle proxy.
func TestUsageLiveHandler_KeepaliveIsAComment(t *testing.T) {
	usage := &liveHandlerUsageDouble{}
	handler := newLiveHandler(t, usage)
	// A long poll with a short keepalive, so the comment arrives while the
	// handler is idle rather than after a change.
	handler.SetIntervals(time.Hour, 40*time.Millisecond)

	stream := dialLiveStream(t, liveRoute(handler))
	stream.readHead(t)
	// The first frame is the snapshot; the keepalive follows it.
	_ = stream.readFrame(t)

	line, err := stream.reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading the keepalive: %v", err)
	}
	if !strings.HasPrefix(line, ":") {
		t.Fatalf("keepalive line = %q, want an SSE comment", line)
	}
	if strings.HasPrefix(line, "data:") {
		t.Fatalf("keepalive line = %q, want a comment rather than a frame", line)
	}
}

// TestUsageLiveHandler_SendsAFrameWhenTheStateChanges pins that a change is
// pushed rather than polled: the stream reads again on its cadence and sends a
// frame only when the frame differs from the last one it sent.
func TestUsageLiveHandler_SendsAFrameWhenTheStateChanges(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	usage := &liveHandlerUsageDouble{}
	handler := newLiveHandler(t, usage)

	stream := dialLiveStream(t, liveRoute(handler))
	stream.readHead(t)
	_ = stream.readFrame(t)

	// A request finishes: the next read reports it, so a frame must follow.
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID: "req_new", TS: now, ProviderID: "anthropic", Model: "claude-3",
		TokensIn: 1, TokensOut: 2, CostUSD: "0.00010000", Status: domain.UsageStatusSuccess,
	}, "", now)
	if err != nil {
		t.Fatalf("building the record: %v", err)
	}
	usage.mu.Lock()
	usage.records = []domain.UsageRecord{record}
	usage.mu.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		payload := stream.readFrame(t)
		var frame schema.UsageLiveFrame
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			t.Fatalf("decoding %s: %v", payload, err)
		}
		if len(frame.Recent) == 1 && frame.Recent[0].RequestID == "req_new" {
			if usage.readCount() < 2 {
				t.Fatalf("the stream sent a changed frame after %d reads, want at least 2", usage.readCount())
			}
			return
		}
	}
	t.Fatal("no frame reporting the finished request arrived")
}
