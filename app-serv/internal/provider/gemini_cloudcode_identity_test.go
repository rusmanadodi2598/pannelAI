// Package provider holds the per-provider plugin seam: the connector that decides
// how one provider is authenticated and addressed.
//
// @file      internal/provider/gemini_cloudcode_identity_test.go
// @for       The client identification the Cloud Code Assist endpoint reads, and the payload it refuses to wrap.
// @uses      net/http, strings, testing.
// @reason    The registry document declares a CLI version and an SDK client string for this family and, until this connector, nothing read either one; a request that arrives without them is answered by the vendor as an unknown client. These tests hold the two headers the entry's declared values produce, the product that must send neither, and the refusal a body this connector cannot read earns before anything leaves.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-18
package provider

import (
	"net/http"
	"strings"
	"testing"
)

func TestCloudCodeHeaders_IdentifyTheClientTheEntryDeclares(t *testing.T) {
	entry := cloudcodeEntry("gemini-cli")
	connector := NewCloudCode(entry, CloudCodeGeminiCLI)
	request, err := http.NewRequest(http.MethodPost, cloudcodeBase, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	if err := connector.ApplyAuth(request, cloudcodeCredential(t, "proj-9")); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer ya29.tok" {
		t.Fatalf("Authorization = %q, want the bearer the login minted", got)
	}
	// Both members below have lived in the registry document with no reader at all.
	if got := request.Header.Get("X-Goog-Api-Client"); got != entry.Transport.APIClient {
		t.Fatalf("X-Goog-Api-Client = %q, want the client the entry declares", got)
	}
	if got := request.Header.Get("User-Agent"); got != "gemini-cli/"+entry.Transport.CLIVersion {
		t.Fatalf("User-Agent = %q, want the CLI version the entry declares", got)
	}
}

func TestCloudCodeHeaders_AntigravityDoesNotFabricateAnIdentity(t *testing.T) {
	entry := cloudcodeEntry("antigravity")
	entry.Transport.Headers = map[string]string{"User-Agent": "antigravity/ide/2.11.0 darwin/arm64"}
	connector := NewCloudCode(entry, CloudCodeAntigravity)
	request, err := http.NewRequest(http.MethodPost, cloudcodeBase, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	if err := connector.ApplyAuth(request, cloudcodeCredential(t, "proj-9")); err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}
	if got := request.Header.Get("X-Goog-Api-Client"); got != "" {
		t.Fatalf("X-Goog-Api-Client = %q, want none: this product identifies itself by its IDE user agent", got)
	}
	if got := request.Header.Get("User-Agent"); got != "" {
		t.Fatalf("User-Agent = %q, want the entry's own header to be the only one sent", got)
	}
}

func TestCloudCodeTransform_RejectsABodyThatIsNotAnObject(t *testing.T) {
	entry := cloudcodeEntry("gemini-cli")
	connector := NewCloudCode(entry, CloudCodeGeminiCLI)

	for _, raw := range []string{`not json`, `[{"contents":[]}]`} {
		request := cloudcodeRequest(t, entry, 1, false, "proj-9")
		request.Body = []byte(raw)
		if err := connector.TransformRequest(&request); err == nil {
			t.Fatalf("TransformRequest() for %q returned no error, want the unreadable payload refused", raw)
		}
	}
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func looksLikeUUID(value string) bool {
	blocks := strings.Split(value, "-")
	if len(blocks) != 5 {
		return false
	}
	for index, want := range []int{8, 4, 4, 4, 12} {
		if len(blocks[index]) != want || !isAllHex(blocks[index]) {
			return false
		}
	}
	return true
}

func isAllHex(value string) bool {
	for _, r := range strings.ToLower(value) {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
