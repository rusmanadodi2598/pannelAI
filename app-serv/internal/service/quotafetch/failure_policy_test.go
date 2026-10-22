// The reference's two failure policies, kept apart on purpose.
//
// @file      internal/service/quotafetch/failure_policy_test.go
// @for       Pinning which provider refusals count as a failed read and which are an answer.
// @uses      context, net/http, net/http/httptest, testing.
// @reason    Most reference usage handlers answer a failure with a message; github's and the cloudcode one antigravity reads through throw instead. A fetcher has no caller to throw at, so the split survives as a flag, and if it is lost, a family that errors every tick is recorded as healthy and never backs off. The owner's reference patches for OpenCode, Qoder, and CodeBuddy keep their soft answers; the dispatcher applies the backoff policy to every other family, including transport and 5xx.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSoftAndHardFailureDifferOnlyInWhoIsRaised(t *testing.T) {
	response := usageResponse{status: http.StatusBadGateway, body: []byte("upstream down")}

	soft, softRefused := response.softFailure("Example")
	hard, hardRefused := response.hardFailure("Example")

	if !softRefused || !hardRefused {
		t.Fatalf("refused = %v/%v, want both true for a 502", softRefused, hardRefused)
	}
	if soft.Message != hard.Message {
		t.Fatalf("messages differ: %q vs %q, want the same sentence", soft.Message, hard.Message)
	}
	if soft.Failed {
		t.Error("softFailure() marked the answer failed, which would back off a provider that spoke")
	}
	if !hard.Failed {
		t.Error("hardFailure() did not mark the answer failed, which is the whole point of it")
	}

	// A 2xx is not a failure of either kind: the marker must not be settable by the caller.
	ok := usageResponse{status: http.StatusOK, body: []byte(`{}`)}
	if raised, refused := ok.hardFailure("Example"); refused || raised.Failed {
		t.Fatalf("2xx hardFailure() = %+v refused %v, want no answer at all", raised, refused)
	}
}

func TestFetchGitHubMarksAProviderErrorFailed(t *testing.T) {
	result := fetchGitHub(context.Background(), Credentials{
		AccessToken: "tok", Endpoint: singleStatusServer(t, http.StatusBadGateway, "upstream down"),
	})

	if !result.Failed {
		t.Fatalf("github 502 = %+v, want Failed so the worker backs off", result)
	}
}

func TestFetchOpenCodeLeavesAProviderErrorSoft(t *testing.T) {
	result := fetchOpenCode(openCodeZen)(context.Background(), Credentials{
		APIKey: "key", Endpoint: singleStatusServer(t, http.StatusInternalServerError, "upstream exploded"),
	})

	if result.Failed {
		t.Fatalf("opencode 500 = %+v, want a soft answer: the reference returns a message here", result)
	}
	if result.Message == "" {
		t.Fatal("opencode 500 carried no sentence, want the provider's own words")
	}
}

// TestFetchClaudeMarksAProviderErrorFailed is the new policy: a 5xx on both the
// primary and the legacy read becomes a failed poll, so the worker backs off
// instead of retrying a broken host on the family floor forever.
func TestFetchClaudeMarksAProviderErrorFailed(t *testing.T) {
	result := Fetch(context.Background(), "claude", Credentials{
		AccessToken: "tok", Endpoint: singleStatusServer(t, http.StatusBadGateway, "upstream down"),
	})

	if !result.Failed {
		t.Fatalf("claude 502 = %+v, want Failed so the worker backs off", result)
	}
	if result.Message == "" {
		t.Fatal("claude 502 carried no sentence, want the card text kept")
	}
}

// TestFetchGroqMarksAProviderErrorFailed covers a family that shares the soft
// sentence helper: the sentence stays, but the dispatcher counts the outage.
func TestFetchGroqMarksAProviderErrorFailed(t *testing.T) {
	result := Fetch(context.Background(), "groq", Credentials{
		APIKey: "key", Endpoint: singleStatusServer(t, http.StatusInternalServerError, "upstream down"),
	})

	if !result.Failed {
		t.Fatalf("groq 500 = %+v, want Failed so the worker backs off", result)
	}
}

// TestReferencePatchedFamiliesKeepTheirSoftAnswer pins the owner's note: the
// OpenCode, Qoder, and CodeBuddy quota reads already carry working reference
// patches, so a transient marker must not start a backoff there.
func TestReferencePatchedFamiliesKeepTheirSoftAnswer(t *testing.T) {
	families := []string{"opencode-zen", "opencode-go", "qoder", "qoder-cn", "codebuddy-cn", "codebuddy-intl"}
	for _, family := range families {
		result := countTransientFailure(family, Result{Message: "upstream down", transient: true})
		if result.Failed {
			t.Fatalf("%s marked failed, want the reference soft answer kept", family)
		}
	}
	for _, family := range []string{"claude", "groq", "github"} {
		result := countTransientFailure(family, Result{Message: "upstream down", transient: true})
		if !result.Failed {
			t.Fatalf("%s stayed soft, want the outage counted", family)
		}
	}
}

// TestSoftFailureMarksOnlyOutagesTransient pins the split inside the shared
// sentence helper: a 5xx or 429 is a read to retry, a 4xx is an answer.
func TestSoftFailureMarksOnlyOutagesTransient(t *testing.T) {
	cases := []struct {
		status        int
		wantTransient bool
	}{
		{status: http.StatusBadGateway, wantTransient: true},
		{status: http.StatusServiceUnavailable, wantTransient: true},
		{status: http.StatusTooManyRequests, wantTransient: true},
		{status: http.StatusUnauthorized, wantTransient: false},
		{status: http.StatusForbidden, wantTransient: false},
		{status: http.StatusNotFound, wantTransient: false},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			result, refused := usageResponse{status: tc.status, body: []byte("denied")}.softFailure("Example")
			if !refused {
				t.Fatalf("softFailure(%d) was not refused", tc.status)
			}
			if result.transient != tc.wantTransient {
				t.Fatalf("softFailure(%d) transient = %v, want %v", tc.status, result.transient, tc.wantTransient)
			}
			if result.Failed {
				t.Fatalf("softFailure(%d) marked Failed, want the dispatcher to decide", tc.status)
			}
		})
	}
}

func singleStatusServer(t *testing.T, status int, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL
}
