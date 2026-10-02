// The reference's two failure policies, kept apart on purpose.
//
// @file      internal/service/quotafetch/failure_policy_test.go
// @for       Pinning which provider refusals count as a failed read and which are an answer.
// @uses      context, net/http, net/http/httptest, testing.
// @reason    Most reference usage handlers answer a failure with a message; github's and the
//
//	cloudcode one antigravity reads through throw instead. A fetcher has no caller to throw at,
//	so the split survives as a flag — and if it is lost, a family that errors every tick is
//	recorded as healthy and never backs off. That is a silent failure, so it is tested directly.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
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

func singleStatusServer(t *testing.T, status int, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL
}
