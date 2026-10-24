// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_sessions_stall_test.go
// @for       What a stalled session queue reports, and what it stays silent about.
// @uses      bytes, context, encoding/json, log/slog, net/http, net/http/httptest, strings, sync, testing, time, internal/provider, internal/registry.
// @reason    A call that dies waiting for a slot never reached the upstream, so an answer blaming the provider sends the operator to the wrong service, and the whole lane can look unhealthy while every provider call is fine. These tests pin the two facts that make the stall findable: the error names the queue, and the queue writes a line when it is the reason a call was slow.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// stallTransport builds a transport whose logger writes into buf, so a test can
// read what the queue reported without a second seam.
func stallTransport(t *testing.T, server *httptest.Server, buf *bytes.Buffer) *Transport {
	t.Helper()
	connectors, err := provider.NewConnectors(provider.DefaultFactory, &sessionConnector{
		Base:    provider.Base{ID: "opencode-a", Auth: "no_auth", Format: registry.DefaultFormat},
		url:     server.URL,
		session: "ses_opencode-a",
	})
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}
	transport, err := NewTransport(TransportDeps{
		Connectors: connectors,
		Client:     server.Client(),
		Logger:     slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	return transport
}

// TestTransport_QueueTimeoutNamesTheGatewayNotTheUpstream pins the half of the
// diagnosis an operator reads first: a call that gave up in the queue must not
// report that the provider failed to answer.
func TestTransport_QueueTimeoutNamesTheGatewayNotTheUpstream(t *testing.T) {
	var buf bytes.Buffer
	entered := make(chan struct{}, 2)
	hold := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(hold) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-hold
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	t.Cleanup(func() {
		releaseAll()
		server.Close()
	})

	transport := stallTransport(t, server, &buf)
	entry := sessionEntry("opencode-a", server.URL)

	first := make(chan error, 1)
	go func() { first <- doOnce(context.Background(), transport, entry) }()
	<-entered // the first call holds the session

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := transport.Do(ctx, Call{Provider: entry, Model: registry.Model{ID: "m"}, Body: []byte(`{"model":"m"}`)})

	var failure *Error
	if !errors.As(err, &failure) {
		t.Fatalf("Do() error = %v, want a data plane error", err)
	}
	if failure.Code != CodeUpstreamTimeout {
		t.Fatalf("code = %q, want %q: a client branches on this", failure.Code, CodeUpstreamTimeout)
	}
	if !strings.Contains(failure.Message, "session queue") {
		t.Fatalf("message = %q, want it to name the queue the call waited in", failure.Message)
	}
	if strings.Contains(failure.Message, "the upstream did not answer") {
		t.Fatalf("message = %q, want no blame put on an upstream this call never reached", failure.Message)
	}

	releaseAll()
	if err := <-first; err != nil {
		t.Fatalf("first Do() error = %v", err)
	}
	written := buf.String()
	if !strings.Contains(written, "gave up waiting for its upstream session slot") {
		t.Fatalf("the stall wrote nothing about itself: %q", written)
	}
	for _, field := range []string{"provider_id", "waited_ms", "queued_calls"} {
		if !strings.Contains(written, `"`+field+`"`) {
			t.Fatalf("the stall line is missing %q: %s", field, written)
		}
	}
}

// TestSessionQueue_NoteWaitWritesOnlyWhatMatters pins the silence rule: an
// uncontended slot costs the log nothing, or every served request on a free lane
// would write a line the operator never reads.
func TestSessionQueue_NoteWaitWritesOnlyWhatMatters(t *testing.T) {
	cases := []struct {
		name   string
		waited time.Duration
		err    error
		want   bool
	}{
		{name: "a slot handed over at once", waited: time.Microsecond, want: false},
		{name: "a slot that took longer than the bound", waited: sessionWaitWarnBounds + time.Second, want: true},
		{name: "a short wait that gave up", waited: time.Millisecond, err: context.DeadlineExceeded, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			queue := newSessionQueue(slog.New(slog.NewJSONHandler(&buf, nil)))
			queue.noteWait(context.Background(), "opencode", "ep_free", 3, tc.waited, tc.err)

			wrote := buf.Len() > 0
			if wrote != tc.want {
				t.Fatalf("wrote a line = %v, want %v (%s)", wrote, tc.want, buf.String())
			}
			if !wrote {
				return
			}
			var record map[string]any
			if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
				t.Fatalf("the line is not JSON: %v (%s)", err, buf.String())
			}
			if record["queued_calls"] != float64(3) {
				t.Fatalf("queued_calls = %v, want the depth the waiter saw", record["queued_calls"])
			}
			if record["endpoint_id"] != "ep_free" {
				t.Fatalf("endpoint_id = %v, want the lane the wait belongs to", record["endpoint_id"])
			}
		})
	}
}
