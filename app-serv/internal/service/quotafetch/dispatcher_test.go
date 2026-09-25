// Dispatcher tests (docs/RULLES/TDD.md §2.5): the map routes by family and answers an
// unknown family with the reference's soft sentence, never a Go error.
//
// @file      internal/service/quotafetch/dispatcher_test.go
// @for       Proves the family map routes each ported family and soft-answers unknown ones.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    The dispatcher is the single entry point callers rely on, so its two outcomes need their own tests.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDispatcher_UnknownFamilyAnswersTheReferenceSentence(t *testing.T) {
	result := Fetch(context.Background(), "groq", Credentials{})

	if result.Message != "Usage API not implemented for groq" {
		t.Fatalf("unknown family message = %q", result.Message)
	}
	if len(result.Quotas) != 0 {
		t.Fatalf("unknown family answered %d quotas, want none", len(result.Quotas))
	}
}

func TestDispatcher_RoutesEachPortedFamilyThroughItsOwnRequest(t *testing.T) {
	cases := []struct {
		name   string
		family string
		path   string
	}{
		{name: "vercel hits the credits endpoint", family: "vercel-ai-gateway", path: "/v1/credits"},
		{name: "codebuddy intl hits the billing endpoint", family: "codebuddy-intl", path: "/v2/billing/meter/get-user-resource"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			paths := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths <- r.URL.Path
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			Fetch(context.Background(), testCase.family, Credentials{APIKey: "k", Endpoint: server.URL})

			if got := <-paths; got != testCase.path {
				t.Fatalf("family %s requested %q, want %q", testCase.family, got, testCase.path)
			}
		})
	}
}
