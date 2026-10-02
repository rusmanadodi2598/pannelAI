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

// TestDispatcher_UnknownFamilyAnswersTheReferenceSentence names a family the registry
// does not carry at all, because every registry provider that publishes a quota now has
// a real handler (see TestFamilyFetchersMatchRegistryUsageProviders).
func TestDispatcher_UnknownFamilyAnswersTheReferenceSentence(t *testing.T) {
	result := Fetch(context.Background(), "not-a-registered-provider", Credentials{})

	if result.Message != "Usage API not implemented for not-a-registered-provider" {
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
		{name: "codebuddy intl hits the billing endpoint", family: "codebuddy-intl", path: "/v2/billing/meter/get-user-resource"},
		{name: "qoder hits the quota usage endpoint", family: "qoder", path: "/api/v2/quota/usage"},
		{name: "groq asks the models endpoint for its headers", family: "groq", path: "/openai/v1/models"},
		{name: "github hits the copilot entitlement endpoint", family: "github", path: "/copilot_internal/user"},
		{name: "deepseek hits the balance endpoint", family: "deepseek", path: "/user/balance"},
		{name: "glm hits the quota limit endpoint", family: "glm", path: "/api/monitor/usage/quota/limit"},
		{name: "opencode zen hits the usage endpoint", family: "opencode-zen", path: "/zen/v1/usage"},
		{name: "opencode go hits its own usage endpoint", family: "opencode-go", path: "/zen/go/v1/usage"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			paths := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case paths <- r.URL.Path:
				default:
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			Fetch(context.Background(), testCase.family, Credentials{
				APIKey:      "k",
				AccessToken: "t",
				Endpoint:    server.URL,
			})

			select {
			case got := <-paths:
				if got != testCase.path {
					t.Fatalf("family %s requested %q, want %q", testCase.family, got, testCase.path)
				}
			default:
				t.Fatalf("family %s made no outbound call at all", testCase.family)
			}
		})
	}
}
