// Tests for the registry usage block's mapping into the quota fetcher.
//
// @file      internal/service/quota_usage_endpoints_test.go
// @for       Pins that every host a provider declares reaches the fetcher that asks for it.
// @uses      internal/registry, internal/service/quotafetch, reflect, testing.
// @reason    The published read once forwarded only `transport.usage.url`, so a family reading `quota_url`, `quota_api_url`, `urls`, `oauth_url` or `user_url` was handed an empty address and called nothing. These cases are why that cannot happen quietly to a key the registry already spells out, or to one it adds later.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"reflect"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// fullUsageBlock sets every declared member to a value distinct enough to tell apart, so
// a mis-wired mapping shows up as the wrong host rather than as a passing empty one.
func fullUsageBlock() registry.UsageConfig {
	value := func(name string) string { return "https://" + name + ".example" }
	return registry.UsageConfig{
		URL:                    value("url"),
		URLs:                   []string{value("urls-first"), value("urls-second")},
		TokenURL:               value("token"),
		QuotaURL:               value("quota"),
		QuotaAPIURL:            value("quota-api"),
		LoadCodeAssistURL:      value("code-assist"),
		LoadProjectAPIURL:      value("project-api"),
		OrgURL:                 value("org"),
		SettingsURL:            value("settings"),
		LimitsPath:             "/limits",
		ResetCreditsConsumeURL: value("credits-consume"),
		ResetCreditsURL:        value("credits"),
		QuotaSummaryAPIURL:     value("summary"),
		UserURL:                value("user"),
		OAuthURL:               value("oauth"),
		CWHost:                 value("cw"),
		QHost:                  value("q"),
	}
}

func TestUsageEndpointsCarriesEveryDeclaredHost(t *testing.T) {
	declared := fullUsageBlock()
	mapped := usageEndpoints(declared)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"url", mapped.URL, declared.URL},
		{"token_url", mapped.TokenURL, declared.TokenURL},
		{"quota_url", mapped.QuotaURL, declared.QuotaURL},
		{"quota_api_url", mapped.QuotaAPIURL, declared.QuotaAPIURL},
		{"load_code_assist_url", mapped.LoadCodeAssistURL, declared.LoadCodeAssistURL},
		{"load_project_api_url", mapped.LoadProjectAPIURL, declared.LoadProjectAPIURL},
		{"org_url", mapped.OrgURL, declared.OrgURL},
		{"settings_url", mapped.SettingsURL, declared.SettingsURL},
		{"limits_path", mapped.LimitsPath, declared.LimitsPath},
		{"reset_credits_consume_url", mapped.ResetCreditsConsume, declared.ResetCreditsConsumeURL},
		{"reset_credits_url", mapped.ResetCredits, declared.ResetCreditsURL},
		{"quota_summary_api_url", mapped.QuotaSummaryAPIURL, declared.QuotaSummaryAPIURL},
		{"user_url", mapped.UserURL, declared.UserURL},
		{"oauth_url", mapped.OAuthURL, declared.OAuthURL},
		{"cw_host", mapped.CWHost, declared.CWHost},
		{"q_host", mapped.QHost, declared.QHost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("mapped %s = %q, want the declared %q", tc.name, tc.got, tc.want)
			}
		})
	}

	if len(mapped.URLs) != 2 || mapped.URLs[0] != declared.URLs[0] || mapped.URLs[1] != declared.URLs[1] {
		t.Fatalf("mapped urls = %v, want the declared pair %v", mapped.URLs, declared.URLs)
	}
}

// TestUsageBlockAndMirrorHaveEqualWidth is the drift tripwire: a key added to the
// registry's usage block without a matching member in the fetcher's mirror trips here
// rather than surfacing as a family that silently asks no host.
func TestUsageBlockAndMirrorHaveEqualWidth(t *testing.T) {
	declared := reflect.TypeOf(registry.UsageConfig{}).NumField()
	mirror := reflect.TypeOf(quotafetch.UsageEndpoints{}).NumField()
	if declared != mirror {
		t.Fatalf("registry.UsageConfig has %d members, the fetcher's mirror has %d; one gained a usage host the other never reads", declared, mirror)
	}
}

// TestPublishedAccountDataHandsTheProjectID pins the mapping itself: a Gemini or Antigravity
// account that stored its project, email or user id is lowered into the few keys a provider's
// usage endpoint names, with blanks dropped rather than sent as empty strings.
//
// It does NOT prove the bootstrap call is gone, a mapper can be perfectly tested and never
// called, which is exactly how F6 shipped wrong once (see PORT 010 §3.6). That claim is held by
// TestQuotaService_PublishedUsageCredential, which reads through the fetcher seam and asserts the
// facts arrive on the credentials a real poll would carry.
func TestPublishedAccountDataHandsTheProjectID(t *testing.T) {
	cases := []struct {
		name    string
		cred    *domain.OAuthCredential
		account domain.EndpointAccount
		want    map[string]string
	}{
		{
			name: "a stored project and identity travel together",
			cred: domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{ProjectID: "proj-1", AccountEmail: "op@example.test", AccountID: "user-9"}),
			want: map[string]string{"projectId": "proj-1", "email": "op@example.test", "userId": "user-9"},
		},
		{
			name:    "email falls back to the account row",
			cred:    domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{ProjectID: "proj-1"}),
			account: domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: " acct@example.test "}),
			want:    map[string]string{"projectId": "proj-1", "email": "acct@example.test"},
		},
		{
			name:    "blank values are left out, not sent as empty",
			cred:    domain.RehydrateOAuthCredential(domain.OAuthCredentialInput{ProjectID: "   ", AccountID: ""}),
			account: domain.RehydrateEndpointAccount(domain.EndpointAccountInput{Email: ""}),
			want:    nil,
		},
		{name: "no credential hands no facts", cred: nil, want: nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := publishedAccountData(testCase.cred, testCase.account)
			if len(got) != len(testCase.want) {
				t.Fatalf("publishedAccountData() = %v, want %v", got, testCase.want)
			}
			for key, want := range testCase.want {
				if got[key] != want {
					t.Fatalf("publishedAccountData()[%s] = %q, want %q (full: %v)", key, got[key], want, got)
				}
			}
		})
	}
}
