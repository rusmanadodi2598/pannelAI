// The declared bootstrap path, honoured (docs/PORT/010-PORT-QUOTA-PUBLISHED.md F2).
//
// @file      internal/service/quotafetch/google_paths_test.go
// @for       Pinning that a registry-declared load_code_assist_url moves the project lookup.
// @uses      context, strings, testing.
// @reason    gemini-cli has no quota path of its own to get wrong until it looks its project up,
//
//	so the bootstrap is the one call whose address the registry actually chooses. Declaring it
//	and silently asking the built-in is the defect this family would otherwise repeat quietly.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"strings"
	"testing"
)

func TestFetchGeminiCLIUsesTheDeclaredBootstrapPath(t *testing.T) {
	const (
		builtIn = "/v1internal:loadCodeAssist"
		moved   = "/moved:loadCodeAssist"
	)

	cases := []struct {
		name     string
		endpoint string
		want     []string
	}{
		{
			name:     "no declaration asks the built-in",
			endpoint: "",
			want:     []string{"POST " + builtIn, "POST /v1internal:retrieveUserQuota"},
		},
		{
			name:     "a declared load_code_assist_url moves the lookup",
			endpoint: moved,
			want:     []string{"POST " + moved, "POST /v1internal:retrieveUserQuota"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			replies := geminiReplies()
			replies[moved] = googleReply{body: googleAccountBody}
			stub := newGoogleStub(t, replies)

			result := fetchGeminiCLI(context.Background(), Credentials{
				AccessToken: "ya29.tok", Endpoint: stub.server.URL,
				Endpoints: UsageEndpoints{LoadCodeAssistURL: testCase.endpoint},
			})

			if got := stub.recorded(); strings.Join(got, ",") != strings.Join(testCase.want, ",") {
				t.Fatalf("calls = %v, want %v", got, testCase.want)
			}
			if len(result.Quotas) == 0 {
				t.Fatalf("rows = none, want the buckets behind the declared bootstrap (message %q)",
					result.Message)
			}
		})
	}
}
