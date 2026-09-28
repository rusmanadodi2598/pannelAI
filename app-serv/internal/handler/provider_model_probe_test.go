// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_model_probe_test.go
// @for       HTTP tests for the §7.4 model test routes (draft 017 §4.10, F10).
// @uses      internal/dataplane, net/http, net/http/httptest, testing.
// @reason    These routes answer 200 for a model that failed and 4xx only for a
//
//	request that was not a question — the opposite of most of the
//	management surface. That inversion has to be pinned at the HTTP edge,
//	where the status code is what a client branches on.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-27
package handler

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// withProviderID installs the {provider_id} path value the ServeMux would set.
func withProviderID(fn http.HandlerFunc, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("provider_id", id)
		fn(w, r)
	}
}

func TestProviderModelTestHandler_Model(t *testing.T) {
	cases := []struct {
		name               string
		providerID         string
		body               string
		fail               map[string]error
		wantStatus         int
		wantOK             bool
		wantCode           string
		wantError          string
		wantStatusUpstream int
	}{
		{name: "healthy model", providerID: "openai", body: `{"model_id":"gpt-4o"}`,
			wantStatus: http.StatusOK, wantOK: true},
		{name: "a refused model is still a 200 row", providerID: "openai", body: `{"model_id":"gpt-4o"}`,
			fail:       map[string]error{"openai/gpt-4o": &dataplane.Error{Code: dataplane.CodeRateLimited, Message: "quota spent", Status: 429}},
			wantStatus: http.StatusOK, wantCode: dataplane.CodeRateLimited, wantError: "quota spent", wantStatusUpstream: 429},
		{name: "a model the pipeline cannot route", providerID: "openai", body: `{"model_id":"gpt-4o"}`,
			fail:       map[string]error{"openai/gpt-4o": &dataplane.Error{Code: dataplane.CodeModelNotFound, Message: "no route"}},
			wantStatus: http.StatusOK, wantCode: dataplane.CodeModelNotFound},
		{name: "missing body", providerID: "openai", wantStatus: http.StatusBadRequest},
		{name: "blank model id", providerID: "openai", body: `{"model_id":"  "}`, wantStatus: http.StatusBadRequest},
		{name: "malformed body", providerID: "openai", body: `{"model_id":`, wantStatus: http.StatusBadRequest},
		{name: "unknown provider", providerID: "ghost", body: `{"model_id":"gpt-4o"}`, wantStatus: http.StatusNotFound},
		{name: "blank provider path", providerID: "", body: `{"model_id":"gpt-4o"}`, wantStatus: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			f.prober.fail = tc.fail
			rr := do(t, http.MethodPost, "/api/v1/providers/"+tc.providerID+"/models/test", tc.body,
				withProviderID(f.modelTest.Model, tc.providerID))
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			body := decodeBody(t, rr)
			if body["model_id"] != "gpt-4o" {
				t.Fatalf("model_id = %v, want gpt-4o", body["model_id"])
			}
			if body["ok"] != tc.wantOK {
				t.Fatalf("ok = %v, want %v", body["ok"], tc.wantOK)
			}
			errorCode, _ := body["error_code"].(string)
			if errorCode != tc.wantCode {
				t.Fatalf("error_code = %v, want %q", body["error_code"], tc.wantCode)
			}
			if tc.wantError != "" && body["error"] != tc.wantError {
				t.Fatalf("error = %v, want %q", body["error"], tc.wantError)
			}
			if tc.wantStatusUpstream != 0 && body["status"] != float64(tc.wantStatusUpstream) {
				t.Fatalf("status = %v, want %d", body["status"], tc.wantStatusUpstream)
			}
			if tc.wantCode == "" && body["name"] != "GPT-4o" {
				t.Fatalf("name = %v, want the catalog's own label", body["name"])
			}
		})
	}
}

func TestProviderModelTestHandler_Models(t *testing.T) {
	cases := []struct {
		name        string
		providerID  string
		body        string
		answers     map[string]dataplane.Outcome
		wantStatus  int
		wantTested  float64
		wantTotal   float64
		wantStopped string
	}{
		{name: "whole catalog in one sweep", providerID: "openai",
			wantStatus: http.StatusOK, wantTested: 2, wantTotal: 2},
		{name: "a limit is reported, not hidden", providerID: "openai", body: `{"limit":1}`,
			wantStatus: http.StatusOK, wantTested: 1, wantTotal: 2},
		{name: "an explicit zero limit means the default", providerID: "openai", body: `{"limit":0}`,
			wantStatus: http.StatusOK, wantTested: 2, wantTotal: 2},
		{name: "a provider with one model", providerID: "anthropic",
			wantStatus: http.StatusOK, wantTested: 1, wantTotal: 1},
		{name: "negative limit", providerID: "openai", body: `{"limit":-3}`, wantStatus: http.StatusBadRequest},
		{name: "malformed body", providerID: "openai", body: `{"limit":"many"}`, wantStatus: http.StatusBadRequest},
		{name: "unknown provider", providerID: "ghost", wantStatus: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagementFixture(t)
			if tc.answers != nil {
				f.prober.answers = tc.answers
			}
			rr := do(t, http.MethodPost, "/api/v1/providers/"+tc.providerID+"/test-models", tc.body,
				withProviderID(f.modelTest.Models, tc.providerID))
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			body := decodeBody(t, rr)
			if body["tested"] != tc.wantTested || body["total"] != tc.wantTotal {
				t.Fatalf("budget = %v/%v, want %v/%v", body["tested"], body["total"], tc.wantTested, tc.wantTotal)
			}
			if body["provider_id"] != tc.providerID || body["source"] != service.ModelSourceRegistry {
				t.Fatalf("identity = %v/%v, want %s and the registry",
					body["provider_id"], body["source"], tc.providerID)
			}
			stopped, _ := body["stopped"].(string)
			if stopped != tc.wantStopped {
				t.Fatalf("stopped = %v, want %q", body["stopped"], tc.wantStopped)
			}
			rows, ok := body["results"].([]any)
			if !ok || len(rows) != int(tc.wantTested) {
				t.Fatalf("results = %v, want %v rows", body["results"], tc.wantTested)
			}
			for _, raw := range rows {
				row, _ := raw.(map[string]any)
				if row["model_id"] == "" || row["model_id"] == nil {
					t.Fatalf("row %v has no model_id", row)
				}
			}
		})
	}
}
