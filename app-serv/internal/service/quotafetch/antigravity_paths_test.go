// The declared usage paths, honoured end to end (docs/PORT/010-PORT-QUOTA-PUBLISHED.md F2).
//
// @file      internal/service/quotafetch/antigravity_paths_test.go
// @for       Pinning that a registry-declared usage path replaces the built-in one.
// @uses      context, net/http/httptest, strings, testing.
// @reason    This family is the reason the plumbing exists. It reads three different paths, each
//
//	of which the registry may spell under its own key, and the original defect was that only one
//	of those keys ever reached the fetcher, so a provider that moved a path got an empty URL and
//	answered nothing while every test stayed green. A built-in-only test cannot see that.
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

func TestFetchAntigravityUsesTheDeclaredUsagePaths(t *testing.T) {
	const (
		loadPath    = "/v1internal:loadCodeAssist"
		modelsPath  = "/v1internal:fetchAvailableModels"
		summaryPath = "/v1internal:retrieveUserQuotaSummary"
		movedLoad   = "/moved:loadCodeAssist"
		movedModels = "/moved:fetchAvailableModels"
		movedSumm   = "/moved:retrieveUserQuotaSummary"
	)

	cases := []struct {
		name      string
		endpoints UsageEndpoints
		want      []string
	}{
		{
			name:      "no declaration asks the built-ins",
			endpoints: UsageEndpoints{},
			want:      []string{"POST " + loadPath, "POST " + modelsPath, "POST " + summaryPath},
		},
		{
			name:      "load_project_api_url moves the account lookup",
			endpoints: UsageEndpoints{LoadProjectAPIURL: movedLoad},
			want:      []string{"POST " + movedLoad, "POST " + modelsPath, "POST " + summaryPath},
		},
		{
			name:      "quota_api_url moves the model read",
			endpoints: UsageEndpoints{QuotaAPIURL: movedModels},
			want:      []string{"POST " + loadPath, "POST " + movedModels, "POST " + summaryPath},
		},
		{
			name:      "quota_summary_api_url moves the weekly read",
			endpoints: UsageEndpoints{QuotaSummaryAPIURL: movedSumm},
			want:      []string{"POST " + loadPath, "POST " + modelsPath, "POST " + movedSumm},
		},
		{
			name: "all three move together",
			endpoints: UsageEndpoints{
				LoadProjectAPIURL: movedLoad, QuotaAPIURL: movedModels, QuotaSummaryAPIURL: movedSumm,
			},
			want: []string{"POST " + movedLoad, "POST " + movedModels, "POST " + movedSumm},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			replies := antigravityReplies(antigravityModelsBody)
			// The moved spellings answer exactly what the built-ins do, so the only thing a case
			// can be wrong about is which path the family chose to dial.
			replies[movedLoad] = googleReply{body: antigravityAccountBody}
			replies[movedModels] = googleReply{body: antigravityModelsBody}
			replies[movedSumm] = replies[summaryPath]
			stub := newGoogleStub(t, replies)

			result := fetchAntigravity(context.Background(), Credentials{
				AccessToken: "ya29.ag", Endpoint: stub.server.URL, Endpoints: testCase.endpoints,
			})

			if got := stub.recorded(); strings.Join(got, ",") != strings.Join(testCase.want, ",") {
				t.Fatalf("calls = %v, want %v", got, testCase.want)
			}
			// Asking the right path is only useful if the answer still becomes rows: a moved path
			// that the family fails to parse would pass the call check and empty the card.
			if len(result.Quotas) == 0 {
				t.Fatalf("rows = none, want the provider's numbers from the declared paths (message %q)",
					result.Message)
			}
		})
	}
}
