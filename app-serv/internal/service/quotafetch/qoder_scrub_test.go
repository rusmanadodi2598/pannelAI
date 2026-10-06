// Package quotafetch reads the published quota of one provider family.
//
// @file      internal/service/quotafetch/qoder_scrub_test.go
// @for       The credential a Qoder 4xx body can quote back before it reaches the card.
// @uses      context, fmt, net/http, net/http/httptest, strings, testing
// @reason    requestUsage scrubs what a call presented because providers quote the request they refused, and the poll worker caches every answer. fetchQoder used to build its own request, so this pins the scrub on the path that replaced it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package quotafetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const qoderScrubToken = "dt-live-device-token-abcdef0123456789"

func TestFetchQoderRedactsTheBearerTheProviderQuotesBack(t *testing.T) {
	var presented string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/quota/usage", func(w http.ResponseWriter, r *http.Request) {
		presented = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"message":"request refused: %s"}`, presented)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	result := fetchQoder("qoder", "")(context.Background(), Credentials{
		Endpoint: server.URL, AccessToken: qoderScrubToken,
	})

	// The guard only means something if the bearer really went out, and really came
	// back inside the provider's own sentence.
	if presented != "Bearer "+qoderScrubToken {
		t.Fatalf("the stub saw %q, want the bearer the credential carries", presented)
	}
	if strings.Contains(result.Message, qoderScrubToken) {
		t.Fatalf("a live bearer reached the cached message: %q", result.Message)
	}
	if !strings.Contains(result.Message, "[redacted]") {
		t.Fatalf("message = %q, want the scrubbed placeholder in the provider's sentence", result.Message)
	}
}
