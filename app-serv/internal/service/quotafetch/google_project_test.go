// Project resolution shared by the Google families: the login path that needs the
// account's Cloud project before any quota or inference call can name one.
//
// @file      internal/service/quotafetch/google_project_test.go
// @for       Locks the bootstrap call that answers which Cloud project an access token holds.
// @uses      context, net/http/httptest, strings, testing.
// @reason    A Google account is metered and routed against a project the token response never names, so the login has to ask a second endpoint for it. The shape worth pinning is the one that is easy to get wrong silently: which endpoint is asked, what the caller identifies itself with, and that an answer without a project is reported as no project rather than as success.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package quotafetch

import (
	"context"
	"strings"
	"testing"
)

func TestLoadProject_AskTheDeclaredEndpointWithTheDeclaredIdentity(t *testing.T) {
	stub := newGoogleStub(t, map[string]googleReply{
		"/v1internal:loadCodeAssist": {body: googleAccountBody},
	})

	project, ok := LoadProject(context.Background(), Credentials{
		Endpoint:  stub.server.URL,
		Endpoints: UsageEndpoints{LoadCodeAssistURL: "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"},
	}, "ya29.tok", "antigravity/ide/2.11.0 darwin/arm64")

	if !ok || project != "proj-9" {
		t.Fatalf("LoadProject() = (%q, %v), want the project the stub published", project, ok)
	}
	calls := stub.recorded()
	if len(calls) != 1 || calls[0] != "POST /v1internal:loadCodeAssist" {
		t.Fatalf("calls = %v, want exactly the bootstrap", calls)
	}
	if got := stub.headers.Get("User-Agent"); got != "antigravity/ide/2.11.0 darwin/arm64" {
		t.Fatalf("User-Agent = %q, want the identity the entry declares", got)
	}
	if got := stub.headers.Get("Authorization"); got != "Bearer ya29.tok" {
		t.Fatalf("Authorization = %q, want the bearer the login just minted", got)
	}
	// The client identification is what the endpoint refuses a call without.
	if body := stub.sentBody(0); !strings.Contains(body, `"ideType":9`) || !strings.Contains(body, `"pluginType":2`) {
		t.Fatalf("body = %q, want the CLIENT_METADATA the bootstrap demands", body)
	}
}

func TestLoadProject_ReportsNoProjectRatherThanAnEmptySuccess(t *testing.T) {
	cases := []struct {
		name  string
		creds Credentials
		token string
		reply googleReply
	}{
		{
			name:  "an answer that carries no project",
			reply: googleReply{body: `{"currentTier":{"name":"free-tier"}}`},
		},
		{
			name:  "a refused lookup",
			reply: googleReply{status: 403, body: `{"error":{"message":"permission denied"}}`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newGoogleStub(t, map[string]googleReply{"/v1internal:loadCodeAssist": tc.reply})
			project, ok := LoadProject(context.Background(), Credentials{
				Endpoint:  stub.server.URL,
				Endpoints: UsageEndpoints{LoadCodeAssistURL: "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"},
			}, "ya29.tok", "")

			if ok || project != "" {
				t.Fatalf("LoadProject() = (%q, %v), want no project and no claim of one", project, ok)
			}
		})
	}
}

func TestLoadProject_SendsNothingWithoutAToken(t *testing.T) {
	stub := newGoogleStub(t, map[string]googleReply{"/v1internal:loadCodeAssist": {body: googleAccountBody}})

	if project, ok := LoadProject(context.Background(), Credentials{
		Endpoint:  stub.server.URL,
		Endpoints: UsageEndpoints{LoadCodeAssistURL: "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"},
	}, "", ""); ok || project != "" {
		t.Fatalf("LoadProject() = (%q, %v), want a refusal to ask at all", project, ok)
	}
	if calls := stub.recorded(); len(calls) != 0 {
		t.Fatalf("calls = %v, want none without a token", calls)
	}
}
