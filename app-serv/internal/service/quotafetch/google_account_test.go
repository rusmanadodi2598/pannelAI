// The shared Cloud Code Assist client: the project both products read from either spelling,
// the identification every bootstrap call carries, and the platform enum derived from the host.
//
// @file      internal/service/quotafetch/google_account_test.go
// @for       Locks Google project normalization, the bootstrap identification and the quota URL declaration.
// @uses      internal/service/quotafetch, context, strings, testing
// @reason    The project arrives as a bare id on one connection shape and an object on another,
//
//	and a lookup that forwards the object text as the project name is refused
//	by the endpoint rather than by this package, so the failure would arrive
//	as a provider error instead of as a bug.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package quotafetch

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGoogleStoredProjectNormalizesBothShapes(t *testing.T) {
	cases := []struct {
		name string
		data map[string]string
		want string
	}{
		{name: "a bare id is used as written", data: map[string]string{googleProjectDataKey: "proj-7"}, want: "proj-7"},
		{name: "surrounding space is trimmed", data: map[string]string{googleProjectDataKey: "  proj-7  "}, want: "proj-7"},
		{name: "an object carrying the id is unwrapped", data: map[string]string{googleProjectDataKey: `{"id":"proj-8"}`}, want: "proj-8"},
		{name: "an object without one names nothing", data: map[string]string{googleProjectDataKey: `{"name":"x"}`}, want: ""},
		{name: "no key at all names nothing", data: nil, want: ""},
	}
	for _, testCase := range cases {
		if got := googleStoredProject(testCase.data); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestGoogleAccountProjectReadsBothPublishedShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "a bare id", raw: `"proj-9"`, want: "proj-9"},
		{name: "an object", raw: `{"id":"proj-9"}`, want: "proj-9"},
		{name: "null", raw: `null`, want: ""},
		{name: "a number, which is no project", raw: `12`, want: ""},
	}
	for _, testCase := range cases {
		got := googleAccountProject(json.RawMessage(testCase.raw))
		if got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestGoogleAssistBodyCarriesTheIdentification(t *testing.T) {
	plain := googleAssistBody(false)
	if !strings.Contains(plain, `"ideType":9`) || !strings.Contains(plain, `"pluginType":2`) {
		t.Errorf("body = %q, want the CLIENT_METADATA identification", plain)
	}
	if strings.Contains(plain, `"mode"`) {
		t.Errorf("body = %q, want no mode flag unless the product sends one", plain)
	}
	if withMode := googleAssistBody(true); !strings.Contains(withMode, `"mode":1`) {
		t.Errorf("body = %q, want the IDE mode flag", withMode)
	}
	if body := googleProjectBody(""); body != "{}" {
		t.Errorf("empty project body = %q, want an object that names nothing", body)
	}
}

func TestGooglePlatformEnumCoversTheHostShapes(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   int
	}{
		{goos: "darwin", goarch: "arm64", want: googlePlatformDarwinARM64},
		{goos: "darwin", goarch: "amd64", want: googlePlatformDarwinAMD64},
		{goos: "linux", goarch: "arm64", want: googlePlatformLinuxARM64},
		{goos: "linux", goarch: "amd64", want: googlePlatformLinuxAMD64},
		{goos: "windows", goarch: "amd64", want: googlePlatformWindowsAMD64},
		{goos: "win32", goarch: "amd64", want: googlePlatformWindowsAMD64},
		{goos: "freebsd", goarch: "amd64", want: googlePlatformUnspecified},
	}
	for _, testCase := range cases {
		if got := googlePlatformEnum(testCase.goos, testCase.goarch); got != testCase.want {
			t.Errorf("%s/%s = %d, want %d", testCase.goos, testCase.goarch, got, testCase.want)
		}
	}
	if meta := googleClientMetadata(); meta.IdeType != googleIDETypeAntigravity || meta.PluginType != googlePluginTypeGemini {
		t.Errorf("metadata = %+v, want the antigravity IDE and the gemini plugin type", meta)
	}
}

// The declared address wins outright: a read that kept the built-in path would ask the stub for
// a path this entry no longer serves.
func TestGeminiCLIReadsItsOwnDeclaredURLs(t *testing.T) {
	stub := newGoogleStub(t, map[string]googleReply{"/moved:retrieveUserQuota": {body: geminiQuotaBody}})

	fetchGeminiCLI(context.Background(), Credentials{
		AccessToken:          "t",
		ProviderSpecificData: map[string]string{googleProjectDataKey: "p"},
		Endpoints:            UsageEndpoints{QuotaURL: stub.server.URL + "/moved:retrieveUserQuota"},
	})

	if got := stub.recorded(); len(got) != 1 || got[0] != "POST /moved:retrieveUserQuota" {
		t.Fatalf("calls = %v, want the declared quota path only", got)
	}
}
