// Package provider holds the per-provider plugin seam: the connector that decides
// how one provider is authenticated and addressed.
//
// @file      internal/provider/gemini_test.go
// @for       The Gemini generateContent endpoint: where the model goes in the URL, and which header the credential rides.
// @uses      net/http, strings, testing, internal/registry.
// @reason    Gemini names the model in the request path rather than in the body, and its two credential kinds are read from different headers, so the two facts this connector owns are exactly the two the OpenAI fallback cannot supply. The path rule matters beyond shape: a model value that is not one segment would address a different endpoint than the entry declares, which is a destination choice rather than a formatting detail.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-17
package provider

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

const geminiBase = "https://generativelanguage.googleapis.com/v1beta/models"

func geminiEntry() registry.Provider {
	return registry.Provider{
		ID: "gemini",
		Transport: registry.Transport{
			Format:  "gemini",
			BaseURL: geminiBase,
			Auth: registry.AuthConfig{
				APIKey: &registry.AuthScheme{Header: "x-goog-api-key", Scheme: "raw"},
				OAuth:  &registry.AuthScheme{Header: "Authorization", Scheme: "bearer"},
			},
		},
	}
}

func geminiRequest(entry registry.Provider, model string, stream bool) Request {
	return Request{
		Provider: entry,
		Wire:     "gemini",
		Model:    registry.Model{ID: model},
		Stream:   stream,
		Headers:  http.Header{},
		Body:     []byte(`{"contents":[{"role":"user","parts":[{"text":"ping"}]}]}`),
	}
}

func TestGeminiEndpoint_PutsTheModelInThePathAndChoosesTheAction(t *testing.T) {
	entry := geminiEntry()
	connector := NewGemini(entry)

	cases := []struct {
		name   string
		stream bool
		want   string
	}{
		{name: "a JSON answer", want: geminiBase + "/gemini-2.5-flash:generateContent"},
		{name: "a stream", stream: true, want: geminiBase + "/gemini-2.5-flash:streamGenerateContent?alt=sse"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := geminiRequest(entry, "gemini-2.5-flash", tc.stream)
			url, err := connector.Endpoint(request, Credential{})
			if err != nil {
				t.Fatalf("Endpoint() error = %v", err)
			}
			if url != tc.want {
				t.Fatalf("url = %q, want %q", url, tc.want)
			}
		})
	}
}

func TestGeminiEndpoint_UsesTheUpstreamIdTheEntryDeclares(t *testing.T) {
	entry := geminiEntry()
	request := geminiRequest(entry, "gemini-3.8-flash", false)
	request.Model = registry.Model{ID: "gemini-3.8-flash", UpstreamModelID: "gemini-3.8-flash-preview"}

	url, err := NewGemini(entry).Endpoint(request, Credential{})
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if !strings.HasSuffix(url, "/gemini-3.8-flash-preview:generateContent") {
		t.Fatalf("url = %q, want the id the entry declares rather than the name the client typed", url)
	}
}

// TestGeminiEndpoint_RefusesAModelThatIsNotOnePathSegment pins the guard that
// keeps the addressed endpoint inside the entry's own base: a value carrying a
// separator, a query, or a fragment would move the request somewhere the registry
// never declared.
func TestGeminiEndpoint_RefusesAModelThatIsNotOnePathSegment(t *testing.T) {
	entry := geminiEntry()
	connector := NewGemini(entry)

	for _, model := range []string{"../v1/internal:generateContent", "a/b", "?key=secret", "x#frag", " ", ""} {
		request := geminiRequest(entry, model, false)
		if _, err := connector.Endpoint(request, Credential{}); err == nil {
			t.Fatalf("Endpoint() for model %q returned no error, want the value refused", model)
		}
	}
}

func TestGeminiEndpoint_ReportsAnEntryWithoutABase(t *testing.T) {
	entry := geminiEntry()
	entry.Transport.BaseURL = ""

	if _, err := NewGemini(entry).Endpoint(geminiRequest(entry, "gemini-2.5-flash", false), Credential{}); err == nil {
		t.Fatal("Endpoint() with no base_url returned no error")
	}
}

// TestGeminiAuth_ReadsEachCredentialFromTheHeaderTheEntryNames pins the half the
// connector does not have to write itself: the per-family auth the registry
// declares, kept working through the fallback it inherits.
func TestGeminiAuth_ReadsEachCredentialFromTheHeaderTheEntryNames(t *testing.T) {
	entry := geminiEntry()
	connector := NewGemini(entry)

	t.Run("a static key", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodPost, geminiBase, nil)
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		if err := connector.ApplyAuth(request, StaticKey("ep-1", "key-1", "AIza-key")); err != nil {
			t.Fatalf("ApplyAuth() error = %v", err)
		}
		if got := request.Header.Get("x-goog-api-key"); got != "AIza-key" {
			t.Fatalf("x-goog-api-key = %q, want the key raw", got)
		}
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q, want no bearer for a key the vendor reads elsewhere", got)
		}
	})

	t.Run("an oauth token", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodPost, geminiBase, nil)
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		if err := connector.ApplyAuth(request, OAuthToken("ep-1", "key-1", "ya29.token")); err != nil {
			t.Fatalf("ApplyAuth() error = %v", err)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer ya29.token" {
			t.Fatalf("Authorization = %q, want the bearer", got)
		}
	})
}
