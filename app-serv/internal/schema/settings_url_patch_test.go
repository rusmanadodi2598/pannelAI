// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_url_patch_test.go
// @for       The URL fields of the §7.14 PATCH: empty clears, malformed is
//
//	refused, and the headroom coherence rule still holds after a clear.
//
// @uses      testing, internal/domain.
// @reason    §7.14 makes a partial PATCH the only door a settings write enters
//
//	through, and the panel sends outbound_proxy_url: "" when the operator
//	empties the field: the tag layer used to refuse that with 400, which is
//	the one way an operator removes a stored URL. The tests pin the three
//	faces of one rule — clear accepted, junk refused, coherence kept.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-25
package schema

import (
	"strings"
	"testing"
)

// flagPtr is this file's pointer helper for the boolean pair.
func flagPtr(v bool) *bool { return &v }

// TestValidateStruct_AcceptsAStoredClearedOrAbsentURL pins the tag layer's
// accept side: an empty string clears the stored value, nil leaves it alone,
// and a URL that would dial still passes both URL fields.
func TestValidateStruct_AcceptsAStoredClearedOrAbsentURL(t *testing.T) {
	cases := []struct {
		name  string
		patch PatchSettingsRequest
	}{
		{name: "an emptied outbound proxy url", patch: PatchSettingsRequest{
			Network: &NetworkSettingsPatch{OutboundProxyURL: strPtr("")},
		}},
		{name: "an omitted outbound proxy url", patch: PatchSettingsRequest{
			Network: &NetworkSettingsPatch{OutboundProxyEnabled: flagPtr(true)},
		}},
		{name: "a stored outbound proxy url", patch: PatchSettingsRequest{
			Network: &NetworkSettingsPatch{OutboundProxyURL: strPtr("http://proxy.internal:8080")},
		}},
		{name: "an emptied headroom url", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{URL: strPtr("")}},
		}},
		{name: "a stored headroom url", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{URL: strPtr("https://saver.internal/compress")}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStruct(tc.patch); err != nil {
				t.Fatalf("ValidateStruct() = %v, want the URL field accepted", err)
			}
		})
	}
}

// TestValidateStruct_RefusesAMalformedURL pins the refuse side: a URL the
// dialer cannot speak is still a validation error for both fields, empty or
// not, so loosening the clear rule must not have loosened the URL check.
func TestValidateStruct_RefusesAMalformedURL(t *testing.T) {
	cases := []struct {
		name  string
		patch PatchSettingsRequest
	}{
		{name: "a bare word as the outbound proxy url", patch: PatchSettingsRequest{
			Network: &NetworkSettingsPatch{OutboundProxyURL: strPtr("not-a-url")},
		}},
		{name: "a scheme with no host as the outbound proxy url", patch: PatchSettingsRequest{
			Network: &NetworkSettingsPatch{OutboundProxyURL: strPtr("http://")},
		}},
		{name: "whitespace as the outbound proxy url", patch: PatchSettingsRequest{
			Network: &NetworkSettingsPatch{OutboundProxyURL: strPtr("   ")},
		}},
		{name: "a bare word as the headroom url", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{URL: strPtr("not-a-url")}},
		}},
		{name: "a non-http scheme as the headroom url", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{URL: strPtr("ftp://saver.internal")}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.patch)
			if err == nil {
				t.Fatal("ValidateStruct() = nil, want the malformed URL refused")
			}
			if !strings.Contains(err.Error(), "URL") && !strings.Contains(err.Error(), "url") {
				t.Fatalf("error = %v, want it to name the URL field", err)
			}
		})
	}
}

// TestValidatePatch_KeepsTheHeadroomRuleAfterAClear pins that the coherence
// rule still bites once the tag layer lets an empty URL through: enabling the
// saver with no destination is refused, while clearing a stored URL without
// enabling is stored as the clear it means.
func TestValidatePatch_KeepsTheHeadroomRuleAfterAClear(t *testing.T) {
	cases := []struct {
		name    string
		patch   PatchSettingsRequest
		wantErr string
	}{
		{name: "enabled with a cleared url", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{
				Enabled: flagPtr(true), URL: strPtr(""),
			}},
		}, wantErr: "headroom.url is required when headroom is enabled"},
		{name: "enabled with a stored url", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{
				Enabled: flagPtr(true), URL: strPtr("https://saver.internal"),
			}},
		}},
		{name: "a cleared url without enabling", patch: PatchSettingsRequest{
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{
				Enabled: flagPtr(false), URL: strPtr(""),
			}},
		}},
		{name: "a cleared outbound url beside the headroom group", patch: PatchSettingsRequest{
			Network:    &NetworkSettingsPatch{OutboundProxyURL: strPtr("")},
			TokenSaver: &TokenSaverSettingsPatch{Headroom: &TokenSaverHeadroomPatch{Enabled: flagPtr(false)}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStruct(tc.patch); err != nil {
				t.Fatalf("ValidateStruct() = %v, want the tag layer to pass", err)
			}
			err := ValidatePatch(tc.patch)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidatePatch() = %v, want the clear accepted", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidatePatch() = nil, want the coherence rule to bite")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidatePatch() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
