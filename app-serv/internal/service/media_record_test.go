// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_record_test.go
// @for       The accounting pair one media call writes: one §7.12 usage row and
//
//	one §7.13 request log, under the request's own identifier.
//
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	errors, strings, testing.
//
// @reason    The register's G6 decision (2026-09-19) is that a media call is
//
//	recorded like any other served request, with tokens 0 and the kind's
//	own per-query price. Each row is a table case so a new ending
//	(served, rejected, unreachable) has to state what it records rather
//	than inheriting an untested default.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_RecordsEveryCall pins what one media call writes however
// it ends: exactly one usage row and one request log, sharing the request id and
// carrying the routing identity, the authenticated key, and the outcome.
func TestMediaCallService_RecordsEveryCall(t *testing.T) {
	cases := []struct {
		name       string
		answer     dataplane.MediaResponse
		failure    error
		wantStatus domain.UsageStatus
		wantCode   string
	}{
		{
			name: "a served call", answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)},
			wantStatus: domain.UsageStatusSuccess,
		},
		{
			name:       "a rate-limited upstream",
			answer:     dataplane.MediaResponse{Status: 429, Body: []byte(`{"error":{"message":"slow down"}}`)},
			wantStatus: domain.UsageStatusError, wantCode: dataplane.CodeRateLimited,
		},
		{
			name: "an unreachable upstream", failure: errors.New("connection refused"),
			wantStatus: domain.UsageStatusError, wantCode: dataplane.CodeInternal,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, caller, usage, logs := recordingMediaFixture(t)
			caller.answer = tc.answer
			caller.failure = tc.failure

			call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
			if err != nil {
				t.Fatalf("Prepare() error = %v", err)
			}
			if _, err := svc.Perform(context.Background(), call, dataplane.MediaRequest{Body: []byte(`{}`)}, "gky_recorded"); tc.wantStatus.Success() && err != nil {
				t.Fatalf("Perform() error = %v", err)
			}

			if len(usage.rows) != 1 || len(logs.rows) != 1 {
				t.Fatalf("recorded %d usage and %d log rows, want one of each", len(usage.rows), len(logs.rows))
			}
			row, entry := usage.rows[0], logs.rows[0]
			if row.RequestID != "req_recorded" || entry.RequestID != row.RequestID {
				t.Fatalf("request ids = %q/%q, want the router's on both rows", row.RequestID, entry.RequestID)
			}
			if row.ProviderID != "openai" || row.EndpointID != "ep-openai" || row.Model != "gpt-4o-mini-tts" {
				t.Fatalf("usage identity = %s/%s/%s, want openai/ep-openai/gpt-4o-mini-tts",
					row.ProviderID, row.EndpointID, row.Model)
			}
			if entry.ProviderID != row.ProviderID || entry.EndpointID != row.EndpointID || entry.Model != row.Model {
				t.Fatalf("log identity = %s/%s/%s, want the usage row's", entry.ProviderID, entry.EndpointID, entry.Model)
			}
			if row.GatewayKeyID != "gky_recorded" || entry.GatewayKeyID != "gky_recorded" {
				t.Fatalf("key ids = %q/%q, want the authenticated key", row.GatewayKeyID, entry.GatewayKeyID)
			}
			if row.Status != tc.wantStatus || row.ErrorCode != tc.wantCode {
				t.Fatalf("usage status = %s/%q, want %s/%q", row.Status, row.ErrorCode, tc.wantStatus, tc.wantCode)
			}
			if row.LatencyMS < 0 || entry.LatencyMS != row.LatencyMS {
				t.Fatalf("latency = %d/%d, want one non-negative measurement", row.LatencyMS, entry.LatencyMS)
			}
			if row.TokensIn != 0 || row.TokensOut != 0 || row.TokensCacheRead != 0 || row.TokensCacheWrite != 0 {
				t.Fatalf("tokens = %d/%d/%d/%d, want none for a media call",
					row.TokensIn, row.TokensOut, row.TokensCacheRead, row.TokensCacheWrite)
			}
			if row.CostUSD != "0" {
				t.Fatalf("cost = %q, want 0 for a kind without a per-query price", row.CostUSD)
			}
			if tc.wantCode == "" {
				if entry.Status != domain.RequestLogSuccess || entry.Error != "" {
					t.Fatalf("log = %s/%q, want a clean success", entry.Status, entry.Error)
				}
				return
			}
			if entry.Status != domain.RequestLogError || !strings.HasPrefix(entry.Error, tc.wantCode+": ") {
				t.Fatalf("log = %s/%q, want the %s code and its message", entry.Status, entry.Error, tc.wantCode)
			}
		})
	}
}

// TestMediaCallService_RecordsTheSearchPrice pins the one media kind that
// declares a price: the row carries `cost_per_query`, and because a search
// request names no model the row's model is the provider that served it.
func TestMediaCallService_RecordsTheSearchPrice(t *testing.T) {
	svc, caller, usage, _ := recordingMediaFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"web":{"results":[]}}`)}

	if _, _, err := svc.Search(context.Background(), schema.SearchRequest{
		Provider: "brave-search", Query: "golang",
	}, "gky_recorded"); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(usage.rows) != 1 {
		t.Fatalf("recorded %d usage rows, want one", len(usage.rows))
	}
	row := usage.rows[0]
	if row.CostUSD != "0.005" {
		t.Fatalf("cost = %q, want the block's declared 0.005", row.CostUSD)
	}
	if row.ProviderID != "brave-search" || row.Model != "brave-search" {
		t.Fatalf("identity = %s/%s, want the provider serving an unnamed model", row.ProviderID, row.Model)
	}
}

// TestMediaCallService_RecordsAFallbackRequestID pins that a row written
// without a router id still carries one: the column is the row's identity, and
// a blank id would make the row unfindable.
func TestMediaCallService_RecordsAFallbackRequestID(t *testing.T) {
	svc, caller, usage, logs := recordingMediaFixture(t)
	svc.recorder = newDataPlaneRecorder(usage, logs, nil)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}

	call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if _, err := svc.Perform(context.Background(), call, dataplane.MediaRequest{Body: []byte(`{}`)}, ""); err != nil {
		t.Fatalf("Perform() error = %v", err)
	}
	if len(usage.rows) != 1 || usage.rows[0].RequestID == "" {
		t.Fatalf("request id = %q, want a generated one", usage.rows[0].RequestID)
	}
	if logs.rows[0].RequestID != usage.rows[0].RequestID {
		t.Fatalf("log request id = %q, want the usage row's", logs.rows[0].RequestID)
	}
}

// TestMediaCallService_ServesWithoutRecorders is the benign control: a
// deployment that wires no accounting still serves, and the answer is unchanged.
func TestMediaCallService_ServesWithoutRecorders(t *testing.T) {
	svc, caller, router := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}

	call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	answer, err := svc.Perform(context.Background(), call, dataplane.MediaRequest{Body: []byte(`{}`)}, "")
	if err != nil {
		t.Fatalf("Perform() error = %v", err)
	}
	if answer.Status != 200 || string(answer.Body) != `{"ok":true}` {
		t.Fatalf("answer = %d/%s, want the upstream's own", answer.Status, answer.Body)
	}
	if router.successes != 1 {
		t.Fatalf("successes = %d, want the served call counted", router.successes)
	}
}

// TestMediaCallService_RecordsFailuresDoNotFailTheCall pins that a failed
// accounting write is bookkeeping: the client already has its answer, and a row
// the panel is missing must not become an error the client cannot act on.
func TestMediaCallService_RecordsFailuresDoNotFailTheCall(t *testing.T) {
	svc, caller, usage, logs := recordingMediaFixture(t)
	usage.failed = errors.New("the usage database is down")
	logs.failed = errors.New("the log database is down")
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"ok":true}`)}

	call, err := svc.Prepare(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	answer, err := svc.Perform(context.Background(), call, dataplane.MediaRequest{Body: []byte(`{}`)}, "gky_recorded")
	if err != nil {
		t.Fatalf("Perform() error = %v, want the answer to survive a failed write", err)
	}
	if answer.Status != 200 {
		t.Fatalf("answer status = %d, want the upstream's 200", answer.Status)
	}
}
