// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage_live_mapping_test.go
// @for       The live frame's field-by-field mapping from the aggregate.
// @uses      encoding/json, testing, time, internal/domain.
// @reason    The frame's member names are a contract app-ui already parses with
//
//	Zod, and its values are a second contract: a status rendered from the
//	wrong field, or a token count read from the wrong counter, would put
//	a number on screen that no row supports. The cases cover the
//	boundaries TDD.md §2.5 requires, including a zero token count, a
//	large one, and a failed request whose code is the only failure fact
//	the frame carries.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-22
package schema

import (
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestUsageLiveFrame_MapsTheAggregate pins the field-by-field mapping, including
// the boundary values TDD.md §2.5 requires: a zero token count, an empty model,
// and a failed request whose error code is the only failure fact the frame
// carries.
func TestUsageLiveFrame_MapsTheAggregate(t *testing.T) {
	started := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name           string
		markers        []domain.ActiveRequest
		records        []domain.UsageRecord
		errorProvider  string
		wantActiveLen  int
		wantRecentLen  int
		wantErrProv    string
		wantStatus     string
		wantTokensIn   int64
		wantModel      string
		wantErrorCode  string
		wantProviderID string
	}{
		{
			name:    "nothing is running and nothing finished",
			markers: nil, records: nil, errorProvider: "",
			wantActiveLen: 0, wantRecentLen: 0, wantErrProv: "",
		},
		{
			name: "one request is running",
			markers: []domain.ActiveRequest{{
				MarkerID: "m1", RequestID: "req_1", ProviderID: "openai", Model: "gpt-4o", StartedAt: started,
			}},
			wantActiveLen: 1, wantRecentLen: 0, wantProviderID: "openai", wantModel: "gpt-4o",
		},
		{
			name: "a served request finished with zero tokens",
			records: []domain.UsageRecord{usageRecordFixture(t,
				"req_zero", "openai", "gpt-4o", domain.UsageStatusSuccess, 0, 0, "", started)},
			wantActiveLen: 0, wantRecentLen: 1, wantStatus: "success", wantTokensIn: 0,
		},
		{
			name: "a failed request finished and names its code",
			records: []domain.UsageRecord{usageRecordFixture(t,
				"req_fail", "anthropic", "claude-3", domain.UsageStatusError, 12, 34, "UPSTREAM_ERROR", started)},
			wantActiveLen: 0, wantRecentLen: 1, wantStatus: "error", wantTokensIn: 12,
			wantErrorCode: "UPSTREAM_ERROR", wantProviderID: "anthropic",
		},
		{
			name: "a large token count survives the mapping",
			records: []domain.UsageRecord{usageRecordFixture(t,
				"req_large", "openai", "gpt-4o", domain.UsageStatusSuccess, 2147483647, 1000000, "", started)},
			wantRecentLen: 1, wantTokensIn: 2147483647,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame := UsageLiveFrameFrom(tc.markers, tc.records, tc.errorProvider)
			if len(frame.Active) != tc.wantActiveLen {
				t.Fatalf("active = %d entries, want %d", len(frame.Active), tc.wantActiveLen)
			}
			if len(frame.Recent) != tc.wantRecentLen {
				t.Fatalf("recent = %d entries, want %d", len(frame.Recent), tc.wantRecentLen)
			}
			if frame.ErrorProvider != tc.wantErrProv {
				t.Fatalf("error_provider = %q, want %q", frame.ErrorProvider, tc.wantErrProv)
			}
			if tc.wantActiveLen > 0 {
				entry := frame.Active[0]
				if entry.ProviderID != tc.wantProviderID {
					t.Errorf("active provider = %q, want %q", entry.ProviderID, tc.wantProviderID)
				}
				if tc.wantModel != "" && entry.Model != tc.wantModel {
					t.Errorf("active model = %q, want %q", entry.Model, tc.wantModel)
				}
				if entry.StartedAt != Timestamp(started) {
					t.Errorf("active started_at = %q, want %q", entry.StartedAt, Timestamp(started))
				}
			}
			if tc.wantRecentLen > 0 {
				entry := frame.Recent[0]
				if tc.wantStatus != "" && entry.Status != tc.wantStatus {
					t.Errorf("recent status = %q, want %q", entry.Status, tc.wantStatus)
				}
				if entry.TokensIn != tc.wantTokensIn {
					t.Errorf("recent tokens_in = %d, want %d", entry.TokensIn, tc.wantTokensIn)
				}
				if tc.wantErrorCode != "" && entry.ErrorCode != tc.wantErrorCode {
					t.Errorf("recent error_code = %q, want %q", entry.ErrorCode, tc.wantErrorCode)
				}
				if tc.wantProviderID != "" && entry.ProviderID != tc.wantProviderID {
					t.Errorf("recent provider = %q, want %q", entry.ProviderID, tc.wantProviderID)
				}
			}
		})
	}
}

// TestUsageLiveFrame_PreservesOrder pins that the frame does not reorder what
// the read bounded: the active list is oldest first so the longest-running
// request is the first row, and the recent list is newest first, which is the
// order the panel renders.
func TestUsageLiveFrame_PreservesOrder(t *testing.T) {
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	markers := []domain.ActiveRequest{
		{MarkerID: "m1", RequestID: "req_oldest", ProviderID: "openai", StartedAt: base.Add(-30 * time.Second)},
		{MarkerID: "m2", RequestID: "req_middle", ProviderID: "anthropic", StartedAt: base.Add(-20 * time.Second)},
		{MarkerID: "m3", RequestID: "req_newest", ProviderID: "gemini", StartedAt: base.Add(-10 * time.Second)},
	}
	records := []domain.UsageRecord{
		usageRecordFixture(t, "req_newest", "gemini", "m3", domain.UsageStatusSuccess, 1, 1, "", base),
		usageRecordFixture(t, "req_older", "openai", "m1", domain.UsageStatusSuccess, 2, 2, "", base.Add(-time.Minute)),
	}
	frame := UsageLiveFrameFrom(markers, records, "")

	for i, want := range []string{"openai", "anthropic", "gemini"} {
		if frame.Active[i].ProviderID != want {
			t.Fatalf("active[%d] = %q, want %q", i, frame.Active[i].ProviderID, want)
		}
	}
	for i, want := range []string{"gemini", "openai"} {
		if frame.Recent[i].ProviderID != want {
			t.Fatalf("recent[%d] = %q, want %q", i, frame.Recent[i].ProviderID, want)
		}
	}
}

// usageRecordFixture builds one aggregate row through the domain constructor, so
// the mapping is tested against a value the gateway could actually produce
// rather than against a hand-built struct with unexported fields set.
func usageRecordFixture(
	t *testing.T,
	requestID, providerID, model string,
	status domain.UsageStatus,
	tokensIn, tokensOut int64,
	errorCode string,
	ts time.Time,
) domain.UsageRecord {
	t.Helper()
	record, err := domain.NewUsageRecord(domain.UsageRecordInput{
		RequestID:  requestID,
		TS:         ts,
		ProviderID: providerID,
		Model:      model,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		CostUSD:    "0.00100000",
		Status:     status,
		ErrorCode:  errorCode,
	}, "", ts)
	if err != nil {
		t.Fatalf("building the usage record fixture: %v", err)
	}
	return record
}
