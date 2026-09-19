// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_refusal_test.go
// @for       The log row a §7.10 media call leaves when it is refused before any
//
//	upstream attempt.
//
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	testing.
//
// @reason    Register G20: a refusal the client saw never appeared in the Logs
//
//	screen because the media routes wrote rows only after Perform. Each
//	refusal class is a table case — unknown provider, untranslated
//	format, missing base URL, no usable account, and search's own two
//	refusals — so a new refusal path has to state what it records.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// recordingRefusalFixture is the recording fixture plus the router, so a case
// can make selection fail.
func recordingRefusalFixture(t *testing.T) (*MediaCallService, *stubMediaRouter, *stubUsageRecorder, *stubLogRecorder) {
	t.Helper()
	svc, _, router := mediaCallFixture(t)
	usage, logs := &stubUsageRecorder{}, &stubLogRecorder{}
	svc.recorder = newDataPlaneRecorder(usage, logs, func(context.Context) string { return "req_refused" })
	return svc, router, usage, logs
}

// TestMediaCallService_RecordsRefusalsBeforeTheCall pins that a call refused
// before any dial leaves exactly one request log row — no usage row, since
// nothing was spent — carrying the refusal's code and the identity the model
// string names, under the router's request id.
func TestMediaCallService_RecordsRefusalsBeforeTheCall(t *testing.T) {
	cases := []struct {
		name         string
		model        string
		kind         domain.MediaKind
		routerFailed error
		wantCode     string
		wantProvider string
		wantModel    string
	}{
		{
			name: "an unknown provider", model: "nope/voice", kind: domain.MediaKindTTS,
			wantCode: dataplane.CodeModelNotFound, wantProvider: "nope", wantModel: "voice",
		},
		{
			name: "a media format the gateway does not translate", model: "edge-tts/voice", kind: domain.MediaKindTTS,
			wantCode: dataplane.CodeProviderNotRoutable, wantProvider: "edge-tts", wantModel: "voice",
		},
		{
			name: "a provider with no base URL", model: "selfhosted/sd-xl", kind: domain.MediaKindImage,
			wantCode: dataplane.CodeValidation, wantProvider: "selfhosted", wantModel: "sd-xl",
		},
		{
			name: "no usable account", model: "openai/gpt-4o-mini-tts", kind: domain.MediaKindTTS,
			routerFailed: domain.NewNoProviderAvailableError("no account for provider openai"),
			wantCode:     dataplane.CodeNoProvider, wantProvider: "openai", wantModel: "gpt-4o-mini-tts",
		},
		{
			name: "a model string with no provider", model: "bare-model", kind: domain.MediaKindTTS,
			wantCode: dataplane.CodeModelNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, router, usage, logs := recordingRefusalFixture(t)
			router.failure = tc.routerFailed

			if _, err := svc.prepareForCall(context.Background(), tc.model, tc.kind, nil, "gky_refused"); err == nil {
				t.Fatalf("prepareForCall(%q) error = nil, want a refusal", tc.model)
			}

			if len(usage.rows) != 0 {
				t.Fatalf("recorded %d usage rows, want none for a call that never dialed", len(usage.rows))
			}
			if len(logs.rows) != 1 {
				t.Fatalf("recorded %d log rows, want the refusal logged", len(logs.rows))
			}
			entry := logs.rows[0]
			if entry.RequestID != "req_refused" || entry.GatewayKeyID != "gky_refused" {
				t.Fatalf("ids = %q/%q, want the request's own", entry.RequestID, entry.GatewayKeyID)
			}
			if entry.Status != domain.RequestLogError || entry.Error != tc.wantCode {
				t.Fatalf("log = %s/%q, want error/%s alone", entry.Status, entry.Error, tc.wantCode)
			}
			if entry.ProviderID != tc.wantProvider || entry.Model != tc.wantModel {
				t.Fatalf("identity = %q/%q, want %q/%q", entry.ProviderID, entry.Model, tc.wantProvider, tc.wantModel)
			}
			if entry.EndpointID != "" || entry.LatencyMS != 0 {
				t.Fatalf("endpoint/latency = %q/%d, want none for a call that never dialed",
					entry.EndpointID, entry.LatencyMS)
			}
			if entry.RequestBody != "" || entry.ResponseBody != "" {
				t.Fatalf("bodies = %q/%q, want none: the media plane stores no bodies",
					entry.RequestBody, entry.ResponseBody)
			}
		})
	}
}

// TestMediaCallService_SuccessfulPrepareRecordsNothing is the benign control: a
// prepare that resolves records nothing, so the refusal row cannot become a
// row-per-call the accounting pair did not ask for.
func TestMediaCallService_SuccessfulPrepareRecordsNothing(t *testing.T) {
	svc, _, usage, logs := recordingRefusalFixture(t)

	if _, err := svc.prepareForCall(context.Background(), "openai/gpt-4o-mini-tts", domain.MediaKindTTS, nil, "gky_ok"); err != nil {
		t.Fatalf("prepareForCall() error = %v, want a resolved call", err)
	}
	if len(usage.rows) != 0 || len(logs.rows) != 0 {
		t.Fatalf("recorded %d usage / %d log rows, want none before the call runs",
			len(usage.rows), len(logs.rows))
	}
}

// TestMediaCallService_RecordsRefusedSearches covers the search route's two
// refusals before Prepare: an unknown provider (where the provider is known, so
// the row names it) and a request that named no provider at all (where the row
// is named by nothing rather than guessed at).
func TestMediaCallService_RecordsRefusedSearches(t *testing.T) {
	cases := []struct {
		name         string
		req          schema.SearchRequest
		wantCode     string
		wantProvider string
	}{
		{
			name: "an unknown provider", req: schema.SearchRequest{Provider: "nope", Query: "golang"},
			wantCode: dataplane.CodeModelNotFound, wantProvider: "nope",
		},
		{
			name: "no provider named", req: schema.SearchRequest{Query: "golang"},
			wantCode: dataplane.CodeValidation,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, usage, logs := recordingRefusalFixture(t)

			if _, _, err := svc.Search(context.Background(), tc.req, "gky_refused"); err == nil {
				t.Fatalf("Search() error = nil, want a refusal")
			}
			if len(usage.rows) != 0 || len(logs.rows) != 1 {
				t.Fatalf("recorded %d usage / %d log rows, want the refusal logged without usage",
					len(usage.rows), len(logs.rows))
			}
			entry := logs.rows[0]
			if entry.Status != domain.RequestLogError || entry.Error != tc.wantCode {
				t.Fatalf("log = %s/%q, want error/%s", entry.Status, entry.Error, tc.wantCode)
			}
			if entry.ProviderID != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", entry.ProviderID, tc.wantProvider)
			}
		})
	}
}
