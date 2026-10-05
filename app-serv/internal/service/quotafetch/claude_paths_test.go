// The declared Claude usage paths, honoured end to end (docs/PORT/010-PORT-QUOTA-PUBLISHED.md F2).
//
// @file      internal/service/quotafetch/claude_paths_test.go
// @for       Pinning that a registry-declared oauth_url or settings_url moves the call Claude makes.
// @uses      context, net/http, strings, testing.
// @reason    `oauth_url` is the key the original defect turned on: the registry declared it, the
//
//	service forwarded only `url`, and the family dialled an empty address with no sound. Every
//	built-in-only test stayed green through that, so the declared spelling is what has to be asked
//	for here, on the primary read and on the legacy pair it falls back to.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package quotafetch

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestFetchClaudeUsesTheDeclaredUsagePaths(t *testing.T) {
	const (
		primaryPath = "/api/oauth/usage"
		movedOAuth  = "/moved/oauth/usage"
		movedSet    = "/moved/settings"
	)

	cases := []struct {
		name      string
		endpoints UsageEndpoints
		primary   claudeReply
		want      []string
	}{
		{
			name:      "no declaration asks the built-in",
			endpoints: UsageEndpoints{},
			primary:   claudeReply{body: claudeUsageBody},
			want:      []string{"GET " + primaryPath},
		},
		{
			name:      "a declared oauth_url moves the primary read",
			endpoints: UsageEndpoints{OAuthURL: movedOAuth},
			primary:   claudeReply{status: http.StatusNotFound},
			want:      []string{"GET " + movedOAuth},
		},
		{
			name:      "a declared settings_url moves the legacy fallback",
			endpoints: UsageEndpoints{SettingsURL: movedSet},
			primary:   claudeReply{status: http.StatusNotFound},
			want:      []string{"GET " + primaryPath, "GET " + movedSet, "GET /v1/organizations/org-7/usage"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			replies := map[string]claudeReply{
				primaryPath:                     testCase.primary,
				movedOAuth:                      {body: claudeUsageBody},
				movedSet:                        {body: claudeSettingsBody},
				"/v1/settings":                  {body: claudeSettingsBody},
				"/v1/organizations/org-7/usage": {body: claudeUsageBody},
			}
			stub := newClaudeStub(t, replies)

			result := fetchClaude(context.Background(), Credentials{
				AccessToken: "tok", Endpoint: stub.server.URL, Endpoints: testCase.endpoints,
			})

			if got := stub.recorded(); strings.Join(got, ",") != strings.Join(testCase.want, ",") {
				t.Fatalf("calls = %v, want %v (message %q)", got, testCase.want, result.Message)
			}
			// The moved address has to still produce the provider's numbers: a path that is asked
			// correctly and then parsed wrongly empties the card while looking pinned.
			if len(result.Quotas) == 0 {
				t.Fatalf("rows = none, want usage from the declared paths (message %q)", result.Message)
			}
		})
	}
}
