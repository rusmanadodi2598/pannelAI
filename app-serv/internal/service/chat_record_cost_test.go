// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/chat_record_cost_test.go
// @for       The cost half of a chat call's usage row: the estimate the rate
//
//	tables produce, and the zero the paths without one must keep.
//
// @uses      internal/dataplane, internal/registry, internal/schema, testing.
// @reason    Live evidence 2026-09-23: every successful chat request wrote
//
//	cost_usd = 0 because chat_record.go never filled the field, so the
//	panel's cost series and the month-to-date spend a budget cap is
//	measured against both read zero. These tests pin the field to the
//	rate tables' estimate for the model that was served, and pin the
//	zero for the cases that must stay zero: an unpriced model, a call
//	that reported no usage, and a call that failed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-23
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestChatService_RecordPricesTheServedCall pins the cost the usage row
// carries: the rate tables' estimate for the served model and the tokens the
// upstream reported, with the zero kept for an unpriced model and for a call
// that reported no usage at all.
func TestChatService_RecordPricesTheServedCall(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		usage   *schema.Usage
		want    string
		wantSet bool
	}{
		{
			// 65 in at $0.14/M + 68 out at $0.28/M = 0.00002814
			name:    "pattern-priced model carries its estimate",
			model:   "deepseek-v4.1-flash:free",
			usage:   &schema.Usage{PromptTokens: 65, CompletionTokens: 68},
			want:    "0.00002814",
			wantSet: true,
		},
		{
			// cache-inclusive prompt: 30 full + 20 cached + 10 creation
			name:    "cached tokens price at the cached rate",
			model:   "deepseek-v4.1-flash",
			usage:   &schema.Usage{PromptTokens: 60, CompletionTokens: 10, PromptTokensDetails: &schema.PromptTokensDetails{CachedTokens: 20, CacheCreationTokens: 10}},
			want:    "0.00000846",
			wantSet: true,
		},
		{
			name:    "unpriced model keeps the zero estimate",
			model:   "totally-unknown-model",
			usage:   &schema.Usage{PromptTokens: 1000, CompletionTokens: 1000},
			want:    "0.00000000",
			wantSet: false,
		},
		{
			name:    "a call that reported no usage keeps zero",
			model:   "deepseek-v4.1-flash",
			usage:   nil,
			want:    "0.00000000",
			wantSet: false,
		},
		{
			name:    "an empty report keeps zero",
			model:   "deepseek-v4.1-flash",
			usage:   &schema.Usage{},
			want:    "0.00000000",
			wantSet: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, usage, _ := chatRecordFixture()
			outcome := dataplane.Outcome{
				ProviderID: "th-1",
				EndpointID: "ep_th",
				Model:      tc.model,
				Usage:      tc.usage,
				LatencyMS:  12,
			}

			svc.record(context.Background(), chatRecordRequest(), outcome, "key_1", "")

			if len(usage.rows) != 1 {
				t.Fatalf("usage rows = %d, want 1", len(usage.rows))
			}
			got := usage.rows[0].CostUSD
			if got != tc.want {
				t.Fatalf("cost_usd = %q, want %q", got, tc.want)
			}
			// A priced call must also have reported tokens: the estimate is a
			// function of them, so a non-zero cost beside zero tokens would mean
			// the field was filled from something other than this call's usage.
			if tc.wantSet && tc.want != "0.00000000" {
				row := usage.rows[0]
				if row.TokensIn+row.TokensOut == 0 {
					t.Fatalf("cost %q recorded beside zero tokens", got)
				}
			}
		})
	}
}

// TestChatService_RecordPricesAFailedCallAtZero pins the failure path: a call
// that failed at the upstream reported no billable usage, so its row carries
// zero rather than a price for work that was not delivered.
func TestChatService_RecordPricesAFailedCallAtZero(t *testing.T) {
	svc, usage, _ := chatRecordFixture()
	outcome := dataplane.Outcome{
		ProviderID: "th-1",
		EndpointID: "ep_th",
		Model:      "deepseek-v4.1-flash",
		LatencyMS:  30,
	}

	svc.record(context.Background(), chatRecordRequest(), outcome, "key_1", dataplane.CodeUpstreamError)

	if len(usage.rows) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(usage.rows))
	}
	if got := usage.rows[0].CostUSD; got != "0.00000000" {
		t.Fatalf("cost_usd = %q, want the zero estimate for a failed call", got)
	}
}
