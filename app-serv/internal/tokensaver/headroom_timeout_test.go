// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/headroom_timeout_test.go
// @for       Headroom refusal and deadline cases.
// @uses      context, encoding/json, net/http, net/http/httptest, testing, time.
// @reason    SPEC-API-001 §7.9 makes the saver fail open and bounds its optional
//
// external call. These cases prove malformed input, a refused proxy,
// an unreachable proxy, and a proxy that answers after the deadline all
// become errors the caller can ignore while keeping the request body.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHeadroomClient_RefusesWhatItCannotSend drives the failures that need no
// answer: an empty or unreadable message array, a URL that cannot be called, and
// a proxy that is not there.
func TestHeadroomClient_RefusesWhatItCannotSend(t *testing.T) {
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := closed.URL
	closed.Close()

	cases := []struct {
		name     string
		url      string
		messages string
	}{
		{name: "no messages at all", url: "http://proxy.local:8787", messages: ""},
		{name: "a messages value that is not JSON", url: "http://proxy.local:8787", messages: `{`},
		{name: "a url that is not absolute", url: "proxy.local:8787", messages: `[]`},
		{name: "a url that is empty", url: "", messages: `[]`},
		{name: "a proxy that is not listening", url: closedURL, messages: `[]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHeadroomClient(http.DefaultClient)
			if _, err := client.Compress(context.Background(), HeadroomRequest{
				URL:      tc.url,
				Model:    "m",
				Messages: json.RawMessage(tc.messages),
			}); err == nil {
				t.Fatal("Compress() error = nil, want a refusal")
			}
		})
	}
}

// TestHeadroomClient_GivesUpOnATimeout proves the call is bounded: a proxy that
// never answers must not hold the request open.
func TestHeadroomClient_GivesUpOnATimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewHeadroomClient(server.Client())
	client.timeout = 20 * time.Millisecond

	started := time.Now()
	_, err := client.Compress(context.Background(), HeadroomRequest{
		URL:      server.URL,
		Model:    "m",
		Messages: json.RawMessage(`[{"role":"user","content":"hi"}]`),
	})
	if err == nil {
		t.Fatal("Compress() error = nil, want a timeout")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("the call took %s, want the 20ms timeout to end it", elapsed)
	}
}

// TestNewHeadroomClient_UsesTheSpecTimeout pins the documented default: §7.9
// fixes 5s, while the reference's own default is 3s. The spec wins, and this
// test is what makes the difference deliberate rather than accidental.
func TestNewHeadroomClient_UsesTheSpecTimeout(t *testing.T) {
	if got := NewHeadroomClient(http.DefaultClient).callTimeout(); got != HeadroomDefaultTimeout {
		t.Fatalf("timeout = %s, want %s", got, HeadroomDefaultTimeout)
	}
	if HeadroomDefaultTimeout != 5*time.Second {
		t.Fatalf("HeadroomDefaultTimeout = %s, want the 5s SPEC-API-001 §7.9 fixes", HeadroomDefaultTimeout)
	}
}
