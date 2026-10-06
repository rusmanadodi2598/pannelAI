// Wire-shape tests for the quota collection's published block.
//
// @file      internal/schema/quota_usage_shape_test.go
// @for       Pins the JSON the quota screen reads: additive published fields, and the three non-percentage states kept apart.
// @uses      encoding/json, internal/schema, testing, time.
// @reason    The provider's number now leads the card, and the panel parses these keys with a strict schema, a renamed or accidentally-always-present field is a screen that errors, not a screen that looks slightly different. Amounts are decimal strings, and an absent ceiling must stay distinguishable from a spent one in the bytes on the wire, because that distinction is the card's whole job.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-02
package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuotaWindowListPublishedIsAdditive(t *testing.T) {
	list := QuotaWindowList{
		Data: []QuotaWindowResponse{{EndpointID: "ep_1", ProviderID: "glm", Window: "5h", Used: 7}},
		Meta: Page{Page: 1, PerPage: 5, Total: 1},
		Published: []PublishedQuotaUsageResponse{{
			EndpointID: "ep_1", ProviderID: "glm", Plan: "Pro", FetchedAt: "2026-10-02T09:00:00Z",
			Cached: true,
			Data: []PublishedQuotaWindowResponse{
				{Label: "Session (5h)", Used: "7", Total: ptrAmount("1000")},
				{Label: "Rolling", Used: "12", Unlimited: true, Unit: "%"},
				{Label: "Balance", Used: "0", Total: ptrAmount("12.5"), Unit: "USD", IsCreditBalance: true},
			},
		}},
	}

	cases := []struct {
		name      string
		wantKey   string
		wantValue string
	}{
		{name: "the published block rides beside data and meta", wantKey: `"published":[`},
		{name: "the provider plan travels", wantKey: `"plan":"Pro"`},
		{name: "a cached answer says so", wantKey: `"cached":true`},
		{name: "amounts are decimal strings", wantKey: `"total":"12.5"`},
		{name: "an unlimited bucket is flagged", wantKey: `"unlimited":true`},
		{name: "a credit balance is flagged", wantKey: `"is_credit_balance":true`},
		{name: "a named unit is carried", wantKey: `"unit":"USD"`},
		{name: "a fetched instant is stamped", wantKey: `"fetched_at":"2026-10-02T09:00:00Z"`},
	}

	encoded, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshaling the quota list: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(string(encoded), tc.wantKey) {
				t.Fatalf("body = %s, want it to contain %s", encoded, tc.wantKey)
			}
		})
	}
	if strings.Contains(string(encoded), `"published_note"`) {
		t.Fatalf("an empty published_note must be omitted, not sent as \"\": %s", encoded)
	}
}

func TestQuotaWindowListPublishedNoteAppearsOnlyOnFailure(t *testing.T) {
	list := QuotaWindowList{
		Data: []QuotaWindowResponse{{EndpointID: "ep_1", ProviderID: "glm", Window: "5h", Used: 7}},
		Meta: Page{Page: 1, PerPage: 5, Total: 1},
	}
	// Whether the route always sends an array is the handler's guarantee, and it is
	// tested there; this file pins what the schema encodes once a value is set.
	list.PublishedNote = "Provider quota could not be read; the counts below are this gateway's own."
	failed, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshaling the degraded quota list: %v", err)
	}
	if !strings.Contains(string(failed), `"published_note":`) {
		t.Fatalf("a cache read failure must be named on the body: %s", failed)
	}
}

func TestPublishedQuotaWindowKeepsNoCeilingDistinctFromZero(t *testing.T) {
	spent, err := json.Marshal(PublishedQuotaWindowResponse{Label: "Weekly", Used: "1000", Total: ptrAmount("0")})
	if err != nil {
		t.Fatalf("marshaling a spent window: %v", err)
	}
	if !strings.Contains(string(spent), `"total":"0"`) {
		t.Fatalf("a ceiling of zero must be sent, not omitted: %s", spent)
	}

	open, err := json.Marshal(PublishedQuotaWindowResponse{Label: "Weekly", Used: "10"})
	if err != nil {
		t.Fatalf("marshaling an unbounded window: %v", err)
	}
	if strings.Contains(string(open), `"total"`) {
		t.Fatalf("an absent ceiling must not be encoded as a value: %s", open)
	}
}

func ptrAmount(value string) *string { return &value }
