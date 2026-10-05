// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_endpoint_test.go
// @for       The host a Qoder credential is served from, and the URL rewrite that
//
//	moves a job token to its own gateway.
//
// @uses      net/http, net/http/httptest, strings, testing, time, internal/registry.
// @reason    The registry entry declares one chat URL, and the vendor serves two
//
//	hosts. Which one a call answers on depends on the credential the
//	account holds, so the rule and its absence (a CN entry, a device
//	token) are pinned here, together with the host fixtures the
//	connector tests share.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"testing"
)

// TestQoderEndpointFollowsTheCredentialKind pins the host rule: a device token keeps

// the declared api3 URL, and a credential that ends up as a job token, a Personal

// Access Token or an already exchanged one, moves to api2 with its path and query

// intact. A CN entry has one gateway and is never rewritten.

func TestQoderEndpointFollowsTheCredentialKind(t *testing.T) {

	intl := qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl)

	cn := qoderEntry("qoder-cn", "https://openapi.qoder.com.cn", qoderChatURLCN)

	var exchanges int

	connector := newQoderTestConnector(t, intl, &exchanges)

	cnConnector := newQoderTestConnector(t, cn, &exchanges)

	cases := []struct {
		name string

		connector *Qoder

		cred Credential

		want string
	}{

		{

			name: "a device token stays on the declared host",

			connector: connector,

			cred: Credential{accessToken: "dt-device", family: FamilyOAuth, projectID: "u"},

			want: qoderChatURLIntl,
		},

		{

			name: "a personal access token moves to the job host",

			connector: connector,

			cred: Credential{apiKey: "pt-secret", family: FamilyStaticKey, projectID: "u"},

			want: "https://api2.qoder.sh/algo/api/v2/service/pro/sse/agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common",
		},

		{

			name: "an exchanged job token moves too",

			connector: connector,

			cred: Credential{accessToken: "jt-known", family: FamilyOAuth, projectID: "u"},

			want: "https://api2.qoder.sh/algo/api/v2/service/pro/sse/agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common",
		},

		{

			name: "a cn entry keeps its single gateway",

			connector: cnConnector,

			cred: Credential{apiKey: "pt-secret", family: FamilyStaticKey, projectID: "u"},

			want: qoderChatURLCN,
		},
	}

	for _, testCase := range cases {

		t.Run(testCase.name, func(t *testing.T) {

			got, err := testCase.connector.Endpoint(Request{}, testCase.cred)

			if err != nil {

				t.Fatalf("Endpoint() error = %v", err)

			}

			if got != testCase.want {

				t.Fatalf("Endpoint() = %q, want %q", got, testCase.want)

			}

		})

	}

}

// TestQoderEndpointMakesNoExchange pins the boundary: choosing a host must not be

// the moment the gateway calls a vendor, so a URL decision stays cheap and a

// transport failure stays attributable to the call that made it.

func TestQoderEndpointMakesNoExchange(t *testing.T) {

	var exchanges int

	connector := newQoderTestConnector(t,

		qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), &exchanges)

	if _, err := connector.Endpoint(Request{}, qoderPATCredential()); err != nil {

		t.Fatalf("Endpoint() error = %v", err)

	}

	if exchanges != 0 {

		t.Fatalf("the endpoint decision exchanged %d times, want none", exchanges)

	}

}

// TestQoderSwapHost pins the rewrite itself: only an exact device host is moved, the

// path and query survive, and anything else is returned untouched rather than

// guessed at.

func TestQoderSwapHost(t *testing.T) {

	cases := []struct {
		name string

		url string

		want string
	}{

		{"the device host moves", "https://api3.qoder.sh/algo/api/v2/model/list",

			"https://api2.qoder.sh/algo/api/v2/model/list"},

		{"the query survives", "https://api3.qoder.sh/algo/x?A=1&B=2", "https://api2.qoder.sh/algo/x?A=1&B=2"},

		{"an already moved host is left alone", "https://api2.qoder.sh/algo/x", "https://api2.qoder.sh/algo/x"},

		{"a lookalike suffix is not rewritten", "https://evil.example/api3.qoder.sh/x",

			"https://evil.example/api3.qoder.sh/x"},

		{"another vendor's host is untouched", "https://gateway.qoder.com.cn/algo/x",

			"https://gateway.qoder.com.cn/algo/x"},
	}

	for _, testCase := range cases {

		t.Run(testCase.name, func(t *testing.T) {

			got, err := qoderSwapHost(testCase.url, qoderChatBaseIntlDevice, qoderChatBaseIntlJob)

			if err != nil {

				t.Fatalf("swapHost() error = %v", err)

			}

			if got != testCase.want {

				t.Fatalf("swapHost(%q) = %q, want %q", testCase.url, got, testCase.want)

			}

		})

	}

}
