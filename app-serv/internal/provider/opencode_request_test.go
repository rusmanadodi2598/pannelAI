// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_request_test.go
// @for       The per-turn x-opencode-request identity app-serv presents on the OpenCode free tier.
// @uses      bytes, crypto/sha256, net/http, strings, testing.
// @reason    The header names one user message, so a retry has to carry the name the first attempt carried, otherwise the free tier counts the retry as a second turn. These tests pin that the id is read from the body that actually leaves, is stable across two reads of it, and survives a body it cannot read without failing the request.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// openCodeRequestFor builds the request shape the transport hands ApplyAuth: a
// body it can re-read, because a retry reads it again.
func openCodeRequestFor(t *testing.T, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/responses", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	return request
}

func TestOpenCodeRequestID_GivesTheCanonicalShape(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"a chat conversation", `{"model":"oc/muse","messages":[{"role":"user","content":"pong"}]}`},
		{"a responses conversation", `{"model":"oc/muse","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"pong"}]}]}`},
		{"a bare input string", `{"model":"oc/muse","input":"pong"}`},
		{"nothing to name", `{"model":"oc/muse","messages":[{"role":"assistant","content":"hi"}]}`},
		{"a body that is not JSON", `not json`},
		{"an empty body", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpenCodeRequestID("ses_000000000000AAAAAAAAAAAAAA", openCodeRequestFor(t, tc.body))
			if !OpenCodeRequestRE.MatchString(got) {
				t.Fatalf("OpenCodeRequestID for %q = %q, does not match %s", tc.body, got, OpenCodeRequestRE)
			}
		})
	}
}

func TestOpenCodeRequestID_RetryOfOneTurnKeepsOneID(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"read the file"}]}`
	first := OpenCodeRequestID("ses_shared", openCodeRequestFor(t, body))
	again := OpenCodeRequestID("ses_shared", openCodeRequestFor(t, body))
	if first != again {
		t.Fatalf("the same turn was named twice: %s then %s", first, again)
	}
}

func TestOpenCodeRequestID_DifferentTurnsGetDifferentIDs(t *testing.T) {
	first := OpenCodeRequestID("ses_shared", openCodeRequestFor(t, `{"messages":[{"role":"user","content":"one"}]}`))
	second := OpenCodeRequestID("ses_shared", openCodeRequestFor(t, `{"messages":[{"role":"user","content":"two"}]}`))
	if first == second {
		t.Fatalf("two different turns share the id %s", first)
	}
}

func TestOpenCodeRequestID_OneSessionPerTurn(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"same words"}]}`
	first := OpenCodeRequestID("ses_a", openCodeRequestFor(t, body))
	second := OpenCodeRequestID("ses_b", openCodeRequestFor(t, body))
	if first == second {
		t.Fatalf("the same message in two sessions shares the id %s", first)
	}
}

func TestOpenCodeRequestID_LastUserMessageNamesTheTurn(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantEquals string
	}{
		{
			name:       "the trailing user message wins over earlier ones",
			body:       `{"messages":[{"role":"user","content":"old"},{"role":"assistant","content":"mid"},{"role":"user","content":"new"}]}`,
			wantEquals: `{"messages":[{"role":"user","content":"new"}]}`,
		},
		{
			name:       "a non-user tail is skipped",
			body:       `{"messages":[{"role":"user","content":"ask"},{"role":"assistant","content":"answer"}]}`,
			wantEquals: `{"messages":[{"role":"user","content":"ask"}]}`,
		},
		{
			name:       "responses parts are joined with one space",
			body:       `{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"a"},{"type":"input_text","text":"b"}]}]}`,
			wantEquals: `{"input":[{"type":"message","role":"user","content":"a b"}]}`,
		},
		{
			name:       "a role-less item is still the user's turn",
			body:       `{"input":[{"content":[{"type":"text","text":"no role"}]}]}`,
			wantEquals: `{"messages":[{"role":"user","content":"no role"}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := "ses_tail" + strings.Repeat("x", 20)
			if got, want := OpenCodeRequestID(session, openCodeRequestFor(t, tc.body)),
				OpenCodeRequestID(session, openCodeRequestFor(t, tc.wantEquals)); got != want {
				t.Fatalf("the same turn was named differently:\n %s from %s\n %s from %s", got, tc.body, want, tc.wantEquals)
			}
		})
	}
}

func TestOpenCodeRequestID_OnlyTheTailOfALongMessageNamesTheTurn(t *testing.T) {
	// The reference names a turn by the last 600 characters of it, so a client
	// that replays a long prompt with different padding is still one turn.
	head := strings.Repeat("z", 700)
	first := OpenCodeRequestID("ses_long", openCodeRequestFor(t, `{"messages":[{"role":"user","content":"`+head+`"}]}`))
	second := OpenCodeRequestID("ses_long", openCodeRequestFor(t, `{"messages":[{"role":"user","content":"`+strings.Repeat("y", 100)+head+`"}]}`))
	if first != second {
		t.Fatalf("two replays of one tail were named differently: %s then %s", first, second)
	}
}

func TestOpenCodeRequestID_UnreadableBodyStillNamesTheTurn(t *testing.T) {
	request := openCodeRequestFor(t, `{"messages":[{"role":"user","content":"ok"}]}`)
	request.GetBody = func() (io.ReadCloser, error) { return nil, errors.New("body gone") }
	if got := OpenCodeRequestID("ses_broken", request); !OpenCodeRequestRE.MatchString(got) {
		t.Fatalf("a body that could not be re-read produced %q, want a canonical id rather than a failure", got)
	}
}

// TestLastOpenCodeUserText pins the reader itself, because the id is only as
// stable as the text it hashes.
func TestLastOpenCodeUserText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"plain messages", `{"messages":[{"role":"user","content":"  pong  "}]}`, "pong"},
		{"a string content with parts", `{"input":[{"type":"message","content":[{"type":"text","text":"one"},{"type":"input_text","text":"two"}]}]}`, "one two"},
		{"an array of bare strings", `{"input":[{"role":"user","content":["a","b"]}]}`, "a b"},
		{"no user message at all", `{"messages":[{"role":"system","content":"be helpful"}]}`, ""},
		{"a body that is not an object", `[1,2,3]`, ""},
		{"an empty object", `{}`, ""},
		{"an input that is an object, not an array or a string", `{"input":{"type":"message"}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastOpenCodeUserText([]byte(tc.body)); got != tc.want {
				t.Fatalf("lastOpenCodeUserText(%s) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}
