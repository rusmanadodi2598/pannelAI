// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/endpoint_bulk_mutation_test.go
// @for       The bulk selector's contract: exactly one way to name a set, and the tags and counts a bulk action refuses.
// @uses      fmt, strings, testing, internal/domain.
// @reason    Every failure here is a bulk action that would otherwise reach the wrong rows, so the table is not a formality: an empty selection that means "everything", a second selector that quietly narrows the first, and a confirm count a client forgot to send are the three ways a mass mutation destroys what nobody pointed at.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-10-16
package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func TestParseBulkSelector(t *testing.T) {
	cases := []struct {
		name    string
		given   BulkSelector
		want    string
		wantTag domain.Tag
		wantAll bool
	}{
		{name: "ids name the set", given: BulkSelector{IDs: []string{" ep_1 ", "ep_2"}}, want: "ids"},
		{name: "a tag names the set", given: BulkSelector{Tag: "Event_Week"}, want: "tag", wantTag: "event_week"},
		{name: "all names the set", given: BulkSelector{All: true}, want: "all", wantAll: true},
		{name: "all narrowed to one provider", given: BulkSelector{All: true, ProviderID: " gemini "}, want: "all", wantAll: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBulkSelector(tc.given)
			if err != nil {
				t.Fatalf("ParseBulkSelector() error = %v", err)
			}
			switch tc.want {
			case "ids":
				if len(got.IDs) != 2 || got.IDs[0] != "ep_1" || got.IDs[1] != "ep_2" {
					t.Fatalf("ids = %v, want the trimmed pair in order", got.IDs)
				}
			case "tag":
				if got.Tag != tc.wantTag {
					t.Fatalf("tag = %q, want %q", got.Tag, tc.wantTag)
				}
			case "all":
				if !got.All {
					t.Fatal("all did not survive the parse")
				}
			}
			if !got.HasTargets() {
				t.Fatal("a resolved set reported itself empty")
			}
			if tc.given.ProviderID != "" && got.ProviderID != strings.TrimSpace(tc.given.ProviderID) {
				t.Fatalf("provider_id = %q, want it trimmed", got.ProviderID)
			}
		})
	}
}

func TestParseBulkSelectorRefusesAnAmbiguousOrEmptySet(t *testing.T) {
	cases := []struct {
		name    string
		given   BulkSelector
		want    string
		message string
	}{
		{
			name:  "nothing named is a refusal, not everything",
			given: BulkSelector{ProviderID: "openai"},
			want:  "BULK_SELECTOR_REQUIRED",
		},
		{
			name:  "ids and a tag name two sets",
			given: BulkSelector{IDs: []string{"ep_1"}, Tag: "free-tier"},
			want:  "BULK_SELECTOR_AMBIGUOUS",
		},
		{
			name:  "ids and all name two sets",
			given: BulkSelector{IDs: []string{"ep_1"}, All: true},
			want:  "BULK_SELECTOR_AMBIGUOUS",
		},
		{
			name:  "a tag and all name two sets",
			given: BulkSelector{Tag: "free-tier", All: true},
			want:  "BULK_SELECTOR_AMBIGUOUS",
		},
		{
			name:  "all three name three sets",
			given: BulkSelector{IDs: []string{"ep_1"}, Tag: "free-tier", All: true},
			want:  "BULK_SELECTOR_AMBIGUOUS",
		},
		{
			name:  "a tag outside the alphabet is refused",
			given: BulkSelector{Tag: "free tier"},
			want:  "VALIDATION_ERROR",
		},
		{
			name:  "a blank id is refused rather than skipped",
			given: BulkSelector{IDs: []string{" "}},
			want:  "VALIDATION_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBulkSelector(tc.given)
			if err == nil {
				t.Fatal("ParseBulkSelector() accepted the request")
			}
			if got := domain.AsAppError(err).Code; got != tc.want {
				t.Fatalf("code = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseBulkSelectorNormalizesTheAuthVocabulary(t *testing.T) {
	cases := []struct {
		given string
		want  domain.UpstreamAuthType
	}{
		{"api_key", domain.UpstreamAuthAPIKey},
		{"apikey", domain.UpstreamAuthAPIKey},
		{"no_auth", domain.UpstreamAuthNone},
		{"none", domain.UpstreamAuthNone},
		{"oauth", domain.UpstreamAuthOAuth},
		{"service_account", domain.UpstreamAuthServiceAccount},
	}
	for _, tc := range cases {
		t.Run(tc.given, func(t *testing.T) {
			got, err := ParseBulkSelector(BulkSelector{All: true, AuthType: tc.given})
			if err != nil {
				t.Fatalf("ParseBulkSelector(%q) error = %v", tc.given, err)
			}
			if got.AuthType != tc.want {
				t.Fatalf("auth type = %q, want %q", got.AuthType, tc.want)
			}
		})
	}
	if _, err := ParseBulkSelector(BulkSelector{All: true, AuthType: "cookie"}); err == nil {
		t.Fatal("a credential the API does not transport was accepted")
	}
}

func TestBulkRequestsValidateAtTheBoundary(t *testing.T) {
	one := "ep_1"
	off := false
	cases := []struct {
		name    string
		given   any
		wantErr bool
	}{
		{"status needs a confirmed count", BulkStatusRequest{Selector: BulkSelector{IDs: []string{one}}, Status: "disabled"}, true},
		{"a confirmed count of zero is nothing seen", BulkStatusRequest{Selector: BulkSelector{IDs: []string{one}}, Status: "disabled", ConfirmCount: 0}, true},
		{"an unknown status is refused", BulkStatusRequest{Selector: BulkSelector{IDs: []string{one}}, Status: "paused", ConfirmCount: 1}, true},
		{"a window past a week is refused", BulkStatusRequest{Selector: BulkSelector{IDs: []string{one}}, Status: "disabled", WindowHours: 200, ConfirmCount: 1}, true},
		{"a full status request passes", BulkStatusRequest{Selector: BulkSelector{IDs: []string{one}}, Status: "disabled", WindowHours: 24, ConfirmCount: 1}, false},
		{"delete needs a confirmed count", BulkDeleteRequest{Selector: BulkSelector{IDs: []string{one}}}, true},
		{"a full delete request passes", BulkDeleteRequest{Selector: BulkSelector{IDs: []string{one}}, ConfirmCount: 1}, false},
		{"tags needs an add or a remove", BulkTagsRequest{Selector: BulkSelector{IDs: []string{one}}, ConfirmCount: 1}, false},
		{"tags past the row bound are refused", BulkTagsRequest{Selector: BulkSelector{IDs: []string{one}}, Add: manyTags(domain.MaxTagsPerRow + 1), ConfirmCount: 1}, true},
		{"a proxy switch to off is a real instruction", BulkProxyStatusRequest{Selector: BulkSelector{IDs: []string{one}}, Enabled: &off, ConfirmCount: 1}, false},
		{"a proxy switch with no value is refused", BulkProxyStatusRequest{Selector: BulkSelector{IDs: []string{one}}, ConfirmCount: 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.given)
			if tc.wantErr && err == nil {
				t.Fatal("ValidateStruct() accepted a request it should refuse")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateStruct() refused a valid request: %v", err)
			}
		})
	}
}

func TestBulkSelectorRejectsATooWideIDList(t *testing.T) {
	ids := make([]string, 0, MaxBulkSelectorIDs+1)
	for range MaxBulkSelectorIDs + 1 {
		ids = append(ids, "ep_1")
	}
	if err := ValidateStruct(BulkSelector{IDs: ids}); err == nil {
		t.Fatal("a list past the id bound was accepted")
	}
}

func manyTags(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("tag-%02d", i))
	}
	return out
}
