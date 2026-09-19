// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/media_provider_test.go
// @for       The media kind set and the base URL shape rule (SPEC-API-001 §7.10).
// @uses      strings, testing, time.
// @reason    The base URL becomes an outbound destination, so its shape is the
//
//	first SSRF layer and needs the parameterized table OWASP §2.5
//	requires: several refusals, plus benign controls that must still
//	be accepted, so an over-blocking rule fails the test too.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-19
package domain

import (
	"strings"
	"testing"
	"time"
)

// TestNewMediaOverride_BaseURLShape pins the shape rule with refusals and
// benign controls. A loopback URL is deliberately ALLOWED here: the shape layer
// must not double as the egress policy, which internal/netguard owns at dial
// time and which an operator can open with EGRESS_ALLOWED_TARGETS.
func TestNewMediaOverride_BaseURLShape(t *testing.T) {
	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		baseURL string
		want    string
		wantErr string
	}{
		{name: "a public https URL", baseURL: "https://api.example.com/v1", want: "https://api.example.com/v1"},
		{name: "a loopback http URL", baseURL: "http://127.0.0.1:8000/v1", want: "http://127.0.0.1:8000/v1"},
		{name: "a bare host", baseURL: "https://api.example.com", want: "https://api.example.com"},
		{name: "a trailing slash", baseURL: "https://api.example.com/v1/", want: "https://api.example.com/v1"},
		{name: "a pasted trailing newline", baseURL: "https://api.example.com/v1\n", want: "https://api.example.com/v1"},
		{name: "empty, meaning the registry default", baseURL: "", want: ""},
		{name: "a file URL", baseURL: "file:///etc/passwd", wantErr: "http or https"},
		{name: "a gopher URL", baseURL: "gopher://api.example.com", wantErr: "http or https"},
		{name: "an ftp URL", baseURL: "ftp://api.example.com", wantErr: "http or https"},
		{name: "a scheme-less host", baseURL: "api.example.com/v1", wantErr: "absolute http(s) URL"},
		{name: "a relative path", baseURL: "/v1", wantErr: "absolute http(s) URL"},
		{name: "an embedded space", baseURL: "https://api.example.com/a b", wantErr: "whitespace"},
		{name: "an embedded tab", baseURL: "https://api.example.com/a\tb", wantErr: "whitespace"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			override, err := NewMediaOverride("openai", MediaKindTTS, tc.baseURL, "gpt-4o-mini-tts", now)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewMediaOverride(%q) accepted, want a refusal naming %q", tc.baseURL, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to name %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewMediaOverride(%q) error = %v", tc.baseURL, err)
			}
			if override.BaseURL() != tc.want {
				t.Fatalf("BaseURL() = %q, want %q", override.BaseURL(), tc.want)
			}
		})
	}
}

// TestParseMediaKind pins the closed kind set: the wire values, the trim and
// case rule, and the refusals — including the panel's own `web` label, which
// the panel maps to `search` before sending and which the API must not accept.
func TestParseMediaKind(t *testing.T) {
	cases := []struct {
		raw     string
		want    MediaKind
		wantErr bool
	}{
		{raw: "tts", want: MediaKindTTS},
		{raw: " TTS ", want: MediaKindTTS},
		{raw: "stt", want: MediaKindSTT},
		{raw: "embedding", want: MediaKindEmbedding},
		{raw: "image", want: MediaKindImage},
		{raw: "video", want: MediaKindVideo},
		{raw: "search", want: MediaKindSearch},
		{raw: "web", wantErr: true},
		{raw: "webSearch", wantErr: true},
		{raw: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			kind, err := ParseMediaKind(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseMediaKind(%q) accepted", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMediaKind(%q) error = %v", tc.raw, err)
			}
			if kind != tc.want {
				t.Fatalf("ParseMediaKind(%q) = %q, want %q", tc.raw, kind, tc.want)
			}
		})
	}
}

// TestNewMediaOverride_Fields pins the identity rules and the stored timestamp.
func TestNewMediaOverride_Fields(t *testing.T) {
	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	override, err := NewMediaOverride(" openai ", MediaKindSearch, "https://api.example.com", " model-a ", now)
	if err != nil {
		t.Fatalf("NewMediaOverride() error = %v", err)
	}
	if override.ProviderID() != "openai" || override.Kind() != MediaKindSearch {
		t.Fatalf("identity = %q/%q, want openai/search", override.ProviderID(), override.Kind())
	}
	if override.DefaultModel() != "model-a" || !override.UpdatedAt().Equal(now) {
		t.Fatalf("model/updatedAt = %q/%v, want model-a/%v", override.DefaultModel(), override.UpdatedAt(), now)
	}

	if _, err := NewMediaOverride("", MediaKindTTS, "", "", now); err == nil {
		t.Fatal("NewMediaOverride() accepted an empty provider id")
	}
	if _, err := NewMediaOverride("openai", "music", "", "", now); err == nil {
		t.Fatal("NewMediaOverride() accepted an unknown kind")
	}
	if _, err := NewMediaOverride("openai", MediaKindTTS, "", strings.Repeat("m", 201), now); err == nil {
		t.Fatal("NewMediaOverride() accepted a default model over 200 characters")
	}
}
