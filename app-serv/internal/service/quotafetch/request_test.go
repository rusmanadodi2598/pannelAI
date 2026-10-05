// Tests for the shared outbound quota read.
//
// @file      internal/service/quotafetch/request_test.go
// @for       Proves a credential this gateway presented never reads back as provider text.
// @uses      internal/service/quotafetch, net/http/httptest, strings, testing.
// @reason    Several providers quote the request they refused inside their 4xx body, and a
//
//	Go transport error carries the request URL. Both end up as a sentence on an
//	operator's card, and the poll worker caches every sentence into the database,
//	so an unscrubbed echo would store a live bearer token on disk for a screen that
//	never needs it. One choke point means no family has to remember this.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestUsageScrubbsEchoedCredential(t *testing.T) {
	// notARealKeyAndHighEntropyOnPurpose: this value must be long enough to clear the scrub
	// minimum and distinctive enough to prove a provider echo was removed, while never
	// resembling a credential a secret scanner would flag in a committed test.
	const secret = "quotatestvalue-0123456789abcdef"

	cases := []struct {
		name    string
		headers map[string]string
		echo    func(r *http.Request) string
	}{
		{
			name:    "bearer echoed in a refused body",
			headers: bearer(secret, nil),
			echo: func(r *http.Request) string {
				return `{"error":{"message":"invalid token ` + "Bearer " + secret + `"}}`
			},
		},
		{
			name:    "bare token echoed without its scheme",
			headers: bearer(secret, nil),
			echo: func(r *http.Request) string {
				return `echoed: ` + secret
			},
		},
		{
			name:    "raw api key header echoed back",
			headers: map[string]string{"x-api-key": secret},
			echo: func(r *http.Request) string {
				return "your key " + secret + " is not valid"
			},
		},
		{
			name:    "credential carried as a query parameter",
			headers: map[string]string{},
			echo: func(r *http.Request) string {
				return "seen " + r.URL.RawQuery
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(testCase.echo(r)))
			}))
			defer server.Close()

			endpoint := server.URL + "/v1/usage"
			if strings.Contains(testCase.name, "query") {
				endpoint += "?api_key=" + secret
			}

			response, err := requestUsage(context.Background(), http.MethodGet, endpoint, testCase.headers, "")
			if err != nil {
				t.Fatalf("requestUsage() error = %v", err)
			}
			if strings.Contains(string(response.body), secret) {
				t.Fatalf("the presented credential survived into the body: %q", response.body)
			}

			failure, refused := response.softFailure("Example")
			if !refused {
				t.Fatal("a 403 must read as a soft failure")
			}
			if strings.Contains(failure.Message, secret) {
				t.Fatalf("the card sentence carries the credential: %q", failure.Message)
			}
		})
	}
}

func TestRequestUsageKeepsNonCredentialIdentification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		// Providers do echo request headers back in their error text; this one echoes
		// its Editor-Version verbatim, and that sentence must survive intact.
		_, _ = w.Write([]byte("Editor-Version " + r.Header.Get("Editor-Version") + " was rejected by the quota API"))
	}))
	defer server.Close()

	headers := map[string]string{"Editor-Version": "vscode/1.90.0"}
	response, err := requestUsage(context.Background(), http.MethodGet, server.URL+"/v1/usage", headers, "")
	if err != nil {
		t.Fatalf("requestUsage() error = %v", err)
	}
	// An identification header is not a secret. Scrubbing it would mangle the very
	// sentence the operator needs in order to fix the request.
	if !strings.Contains(string(response.body), "vscode/1.90.0") {
		t.Fatalf("a non-credential header value was scrubbed: %q", response.body)
	}
}

func TestPresentedSecretsIgnoresShortValues(t *testing.T) {
	secrets := presentedSecrets("https://example.invalid/u?token=abc", map[string]string{
		"Authorization": "Bearer x",
		"X-Product":     "SaaS",
		"Content-Type":  "application/json",
	})
	if len(secrets) != 0 {
		t.Fatalf("presentedSecrets() = %v, want nothing: none of these are secrets worth scrubbing", secrets)
	}
}

// TestSoftFailureDoesNotQuoteMarkup: a moved endpoint answers a 404 with an HTML page, and
// the poll worker stores whatever sentence comes back. Rendering that page on a card would
// bury the one fact the operator needs, that the endpoint is gone.
func TestSoftFailureDoesNotQuoteMarkup(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantSub string
		forbid  string
	}{
		{name: "html is dropped", status: http.StatusNotFound, body: "<!DOCTYPE html><html><body>404</body></html>", wantSub: "quota API error (404)", forbid: "<html>"},
		{name: "a plain-text reason is kept", status: http.StatusServiceUnavailable, body: "gateway unreachable", wantSub: "quota API error (503): gateway unreachable", forbid: ""},
		{name: "a json rejection is quoted", status: http.StatusBadRequest, body: `{"error":{"message":"plan has no usage API"}}`, wantSub: "plan has no usage API", forbid: ""},
		{name: "markup after prose is cut", status: http.StatusInternalServerError, body: "upstream gave up <html><body>trace</body></html>", wantSub: "quota API error (500): upstream gave up", forbid: "<html>"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := usageResponse{status: testCase.status, body: []byte(testCase.body)}
			failure, refused := response.softFailure("Example")
			if !refused {
				t.Fatal("a non-2xx must read as a soft failure")
			}
			if !strings.Contains(failure.Message, testCase.wantSub) {
				t.Fatalf("message = %q, want it to contain %q", failure.Message, testCase.wantSub)
			}
			if testCase.forbid != "" && strings.Contains(failure.Message, testCase.forbid) {
				t.Fatalf("message = %q, must not carry %q", failure.Message, testCase.forbid)
			}
		})
	}
}
