// Kimi wire tests (AGENTS.md §2.1): the exact path, method and credential of the one read, the host it
// falls back to, and every refusal this surface words differently.
//
// @file      internal/service/quotafetch/kimi_endpoints_test.go
// @for       Locks the Kimi request and its soft answers: GET on the usage surface, x-api-key for a stored key, bearer plus device identity for an OAuth connection, and the sentences a refusal becomes.
// @uses      internal/service/quotafetch, net/http/httptest
// @reason    The same endpoint authenticates two ways and answers 403 for two different problems, so a bearer where a raw key belongs and a merged refusal both reach the operator as a dead session.
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

// TestKimi_AsksTheUsageSurfaceWithTheCredentialTheConnectionCarries is the regression that catches this
// family calling the wrong host or shipping the wrong scheme: one read, one call, on the built-in path,
// with the header the endpoint authenticates that credential by.
func TestKimi_AsksTheUsageSurfaceWithTheCredentialTheConnectionCarries(t *testing.T) {
	cases := []struct {
		name   string
		creds  Credentials
		want   map[string]string
		absent []string
	}{
		{
			name:  "a stored api key travels raw, exactly as the platform sends it",
			creds: Credentials{APIKey: " km-1 "},
			want: map[string]string{"x-api-key": "km-1", "Content-Type": "application/json",
				"Accept": "application/json"},
			absent: []string{"Authorization", "X-Msh-Device-Id"},
		},
		{
			name:   "the api key wins when a connection carries both credentials",
			creds:  Credentials{APIKey: "km-1", AccessToken: "tok-1"},
			want:   map[string]string{"x-api-key": "km-1"},
			absent: []string{"Authorization", "X-Msh-Platform"},
		},
		{
			name:  "an oauth connection travels as a bearer beside its device id",
			creds: Credentials{AccessToken: "tok-1", ProviderSpecificData: map[string]string{"deviceId": "dev-9"}},
			want: map[string]string{"Authorization": "Bearer tok-1", "X-Msh-Platform": "9router",
				"X-Msh-Version": "1.0.0", "X-Msh-Device-Id": "dev-9"},
			absent: []string{"x-api-key"},
		},
		{
			name:  "the other spelling of the stored device id is read too",
			creds: Credentials{AccessToken: "tok-1", ProviderSpecificData: map[string]string{"device_id": "dev-8"}},
			want:  map[string]string{"Authorization": "Bearer tok-1", "X-Msh-Device-Id": "dev-8"},
		},
		{
			name:  "a connection authorised on no device names no device",
			creds: Credentials{AccessToken: "tok-1"},
			want: map[string]string{"Authorization": "Bearer tok-1",
				"X-Msh-Platform": "9router", "X-Msh-Version": "1.0.0"},
			absent: []string{"X-Msh-Device-Id", "x-api-key"},
		},
		{
			name: "the headers the entry declares travel, a blank one does not",
			creds: Credentials{APIKey: "km-1",
				UsageHeaders: map[string]string{"X-Trace": "t-1", "X-Blank": "  "}},
			want:   map[string]string{"x-api-key": "km-1", "X-Trace": "t-1"},
			absent: []string{"X-Blank"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := kimiStubFor(t, 0, `{"usage":{"limit":10,"used":1}}`)
			result := stub.read(t, testCase.creds)

			if got := stub.requests(); got != "GET "+kimiUsagePath {
				t.Fatalf("requests = %q, want the one usage surface read once at %q", got, kimiUsagePath)
			}
			if len(result.Quotas) != 1 {
				t.Fatalf("rows = %+v, want the published window (%s)", result.Quotas, result.Message)
			}
			for key, want := range testCase.want {
				if got := stub.header.Get(key); got != want {
					t.Errorf("header %s = %q, want %q", key, got, want)
				}
			}
			for _, key := range testCase.absent {
				if got := stub.header.Get(key); got != "" {
					t.Errorf("header %s = %q, want the family to send nothing for it", key, got)
				}
			}
		})
	}
}

// TestKimi_ReadsItsURLFromTheDeclaredEndpointOrTheBuiltIn pins both halves of the fallback: the host the
// registry entry declares wins outright, and the built-in is only the answer when nothing is declared.
func TestKimi_ReadsItsURLFromTheDeclaredEndpointOrTheBuiltIn(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		wantLine string
	}{
		{name: "the built-in surface when the entry declares no usage url", wantLine: "GET " + kimiUsagePath},
		{name: "the declared usage url wins", declared: "/moved/coding/v1/usages",
			wantLine: "GET /moved/coding/v1/usages"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := kimiStubFor(t, 0, `{"usage":{"limit":10,"used":1}}`)
			creds := Credentials{APIKey: "km-1"}
			if testCase.declared != "" {
				creds.Endpoints.URL = stub.server.URL + testCase.declared
			} else {
				creds.Endpoint = stub.server.URL
			}
			fetchKimi(context.Background(), creds)

			if got := stub.requests(); got != testCase.wantLine {
				t.Fatalf("requests = %q, want %q", got, testCase.wantLine)
			}
		})
	}
	// The seam rewrites the host, so the built-in host is pinned here rather than from a request: the
	// kimi entry declares no usage block, and this is the only host a keyless read can reach.
	if kimiUsageURL != "https://api.kimi.com/coding/v1/usages" {
		t.Errorf("built-in usage url = %q, want the host the reference hard-codes", kimiUsageURL)
	}
}

// TestKimi_SoftAnswers pins the refusals this surface gives: nothing asked without a credential, an
// expired session told apart from an account with no usage entitlement, and an answer that is not json.
func TestKimi_SoftAnswers(t *testing.T) {
	long := strings.Repeat("x", 150)
	cases := []struct {
		name      string
		creds     Credentials
		status    int
		body      string
		wantCalls int64
		want      string
		starts    string
	}{
		{name: "no credential asks the provider nothing", wantCalls: 0,
			want: "Kimi access token or API key not available."},
		{name: "a credential of only spaces is no credential", creds: Credentials{APIKey: "  ", AccessToken: "\t"},
			wantCalls: 0, want: "Kimi access token or API key not available."},
		{name: "a 401 is the session dying, whatever the body says", creds: Credentials{APIKey: "k"}, status: http.StatusUnauthorized,
			body:      `{"code":"16","details":[{"localizedMessage":{"message":"token expired"}}]}`,
			wantCalls: 1, want: "Kimi authentication expired. Please re-authorize."},
		{name: "a 403 with the no-permission reason is not a dead session", creds: Credentials{APIKey: "k"},
			status: http.StatusForbidden, body: `{"code":"7","details":[{"reason":"REASON_FEATURE_NO_PERMISSION"}]}`,
			wantCalls: 1,
			want: "Kimi connected, but this account has no permission to view usage. " +
				"Subscribe to Kimi Code to access quota."},
		{name: "the provider's own sentence is quoted when it has one", creds: Credentials{AccessToken: "tok"},
			status: http.StatusForbidden,
			body: `{"code":"7","details":[{"debug":{"reason":"REASON_FEATURE_NO_PERMISSION"},` +
				`"localizedMessage":{"message":"This plan does not publish usage."}}]}`,
			wantCalls: 1, want: "This plan does not publish usage."},
		{name: "the reason beside the block counts too", creds: Credentials{APIKey: "k"}, status: http.StatusForbidden,
			body: `{"reason":"REASON_FEATURE_NO_PERMISSION"}`, wantCalls: 1,
			want: "Kimi connected, but this account has no permission to view usage. " +
				"Subscribe to Kimi Code to access quota."},
		{name: "a plain refusal naming a subscription is that same answer", creds: Credentials{APIKey: "k"},
			status: http.StatusForbidden, body: `<html>permission_denied: subscribe first</html>`, wantCalls: 1,
			want: "Kimi connected, but this account has no permission to view usage. " +
				"Subscribe to Kimi Code to access quota."},
		{name: "a 403 that is a real dead session says so", creds: Credentials{AccessToken: "tok"},
			status:    http.StatusForbidden,
			body:      `{"code":"7","details":[{"localizedMessage":{"message":"Session was revoked."}}]}`,
			wantCalls: 1, want: "Kimi Coding. API Error 403: Session was revoked."},
		{name: "a server error quotes the body and nothing past a hundred runes", creds: Credentials{APIKey: "k"},
			status: http.StatusInternalServerError, body: long, wantCalls: 1,
			want: "Kimi Coding. API Error 500: " + long[:100]},
		{name: "a server error with nothing to quote is still named", creds: Credentials{APIKey: "k"},
			status: http.StatusServiceUnavailable, wantCalls: 1, want: "Kimi Coding. API Error 503"},
		{name: "a body that is not json stays soft", creds: Credentials{APIKey: "k"},
			body: `<html>login first</html>`, wantCalls: 1, want: "Kimi Coding. Invalid JSON response from API."},
		{name: "an answer that publishes no window says so", creds: Credentials{APIKey: "k"}, body: `{}`,
			wantCalls: 1, want: "Kimi Coding. Usage tracked per request."},
		{name: "an unreachable provider is a sentence, not a panic",
			creds:     Credentials{APIKey: "k", Endpoints: UsageEndpoints{URL: "https://api.kimi.com/coding/v1/usages\x7f"}},
			wantCalls: 0, starts: "Kimi Coding. Unable to fetch usage: "},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := kimiStubFor(t, testCase.status, testCase.body)
			creds := testCase.creds
			if creds.Endpoints.URL == "" {
				creds.Endpoint = stub.server.URL
			}
			result := fetchKimi(context.Background(), creds)

			if got := stub.calls.Load(); got != testCase.wantCalls {
				t.Fatalf("outbound calls = %d, want %d (%s)", got, testCase.wantCalls, stub.requests())
			}
			if len(result.Quotas) != 0 {
				t.Fatalf("rows = %+v, want a soft answer", result.Quotas)
			}
			if result.Plan != "Kimi Coding" {
				t.Errorf("plan = %q, want the family word on a soft answer", result.Plan)
			}
			if testCase.want != "" && result.Message != testCase.want {
				t.Fatalf("message = %q, want %q", result.Message, testCase.want)
			}
			if testCase.starts != "" && !strings.HasPrefix(result.Message, testCase.starts) {
				t.Fatalf("message = %q, want it to start with %q", result.Message, testCase.starts)
			}
		})
	}
}
