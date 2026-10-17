// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/endpoint_bulk_mutation_test.go
// @for       The bulk mutation handler's own two decisions: how a resolved set is answered, and which selector refusals stop before the service is reached.
// @uses      encoding/json, net/http, net/http/httptest, strings, testing, internal/domain.
// @reason    The service resolves the set and the schema parses the selector, but the row-level attribution and the count the operator is shown are this layer's, and they are the parts a client renders. A per-row CONFLICT that loses its code, or an ambiguous selector that reaches the service, both look correct in a service test and wrong in the browser.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-10-16
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

func TestWriteBulkOutcomeAttributesEachRow(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeBulkOutcome(recorder, service.BulkOutcome{
		Matched: 3,
		Changed: 2,
		Results: []service.BulkRowResult{
			{Index: 0, ID: "uky_one"},
			{Index: 1, ID: "uky_two", Error: domain.NewConflictError("this endpoint must keep at least one active key")},
			{Index: 2, ID: "uky_three"},
		},
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the action reported as completed with one refused row", recorder.Code)
	}
	var body schema.BulkMutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	if body.Matched != 3 || body.Changed != 2 {
		t.Fatalf("counts = (%d, %d), want the server's own (3, 2)", body.Matched, body.Changed)
	}
	if len(body.Results) != 3 {
		t.Fatalf("results = %d rows, want one per resolved credential", len(body.Results))
	}
	if body.Results[1].Code != "CONFLICT" || !strings.Contains(body.Results[1].Error, "at least one active key") {
		t.Fatalf("row 1 = %+v, want the refusal's code and message beside the row", body.Results[1])
	}
	if body.Results[0].Error != "" || body.Results[0].Code != "" {
		t.Fatalf("row 0 = %+v, want a landed row to carry no error", body.Results[0])
	}
}

func TestReadSelectorRefusesBeforeTheServiceIsReached(t *testing.T) {
	cases := []struct {
		name    string
		given   schema.BulkSelector
		want    string
		message string
	}{
		{"nothing named", schema.BulkSelector{}, "BULK_SELECTOR_REQUIRED", "select rows by ids"},
		{"two ways at once", schema.BulkSelector{IDs: []string{"ep_1"}, All: true}, "BULK_SELECTOR_AMBIGUOUS", "exactly one of them"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			_, ok := readSelector(recorder, tc.given)
			if ok {
				t.Fatal("readSelector() accepted a selector that names no single set")
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), tc.want) {
				t.Fatalf("body = %s, want the %s code", recorder.Body.String(), tc.want)
			}
		})
	}
}

func TestReadSelectorAcceptsOneNamedSet(t *testing.T) {
	recorder := httptest.NewRecorder()
	sel, ok := readSelector(recorder, schema.BulkSelector{Tag: "Event_Week", AuthType: "apikey"})
	if !ok {
		t.Fatalf("readSelector() refused a valid selector: %s", recorder.Body.String())
	}
	if sel.Tag != "event_week" {
		t.Fatalf("tag = %q, want the canonical spelling the domain owns", sel.Tag)
	}
	if sel.AuthType != domain.UpstreamAuthAPIKey {
		t.Fatalf("auth type = %q, want the registry's spelling mapped to the wire vocabulary", sel.AuthType)
	}
}
