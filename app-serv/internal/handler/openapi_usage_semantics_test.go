// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_usage_semantics_test.go
// @for       The served contract's stated semantics for the aggregate latency
//
//	and the free-text q scope (draft 010 F3 + F8).
//
// @uses      encoding/json, strings, testing.
// @reason    Draft 010 F3: latency_ms on an aggregate is a sum of per-request
//
//	latencies, and nothing on the wire said so, which is why the panel
//	already refused to render it. F8: q matched the model only while
//	the panel's placeholder promised a request id and an error code.
//	Both are statements a consumer reads from the contract, so these
//	tests read the served document rather than the YAML it was
//	generated from: a description lost in regeneration fails here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-22
package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// usageQueryScope is the sentence the three usage reads must carry on q, and
// logQueryScope the one on the logs read. The two differ because a log row
// carries error text where a usage row carries an error code.
const (
	usageQueryScope = "Free-text match over a case-insensitive substring of the id, request_id, error_code, and model fields."
	logQueryScope   = "Free-text match over a case-insensitive substring of the request_id, error, and model fields."
)

// propertyDescription reads one property's description from a served schema,
// failing when the property or its description is absent so a document that
// lost the sentence cannot pass by omission.
func propertyDescription(t *testing.T, schemaName, property string) string {
	t.Helper()
	doc := loadContract(t)
	raw, ok := doc.Components.Schemas[schemaName]
	if !ok {
		t.Fatalf("schema %s is missing from the served document", schemaName)
	}
	var definition struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatalf("decoding schema %s: %v", schemaName, err)
	}
	prop, ok := definition.Properties[property]
	if !ok {
		t.Fatalf("schema %s declares no %q property", schemaName, property)
	}
	if prop.Description == "" {
		t.Fatalf("schema %s property %q carries no description", schemaName, property)
	}
	return prop.Description
}

// TestOpenAPIContract_AggregateLatencyIsStatedAsASum pins the semantic the
// panel already acted on (draft 010 F3, owner decision D1 = document): the
// aggregate field is a sum that grows with the request count, the per-request
// picture is the two percentiles, and the only field that means one
// measurement is the record's own.
func TestOpenAPIContract_AggregateLatencyIsStatedAsASum(t *testing.T) {
	t.Run("UsageTotals.latency_ms states the sum", func(t *testing.T) {
		description := propertyDescription(t, "UsageTotals", "latency_ms")
		for _, want := range []string{
			"Sum of every matched request",
			"grows with the request count",
			"latency_p50_ms",
			"latency_p95_ms",
		} {
			if !strings.Contains(description, want) {
				t.Fatalf("UsageTotals.latency_ms description = %q, want it to contain %q", description, want)
			}
		}
	})

	t.Run("UsageTotals percentiles state the per-request picture", func(t *testing.T) {
		p50 := propertyDescription(t, "UsageTotals", "latency_p50_ms")
		if !strings.Contains(p50, "Median latency over the matched requests") {
			t.Fatalf("latency_p50_ms description = %q, want the median sentence", p50)
		}
		p95 := propertyDescription(t, "UsageTotals", "latency_p95_ms")
		if !strings.Contains(p95, "95th percentile latency over the matched requests") {
			t.Fatalf("latency_p95_ms description = %q, want the percentile sentence", p95)
		}
	})

	t.Run("UsageRecordResponse.latency_ms states one request's own duration", func(t *testing.T) {
		description := propertyDescription(t, "UsageRecordResponse", "latency_ms")
		for _, want := range []string{"This request's own duration", "UsageTotals.latency_ms"} {
			if !strings.Contains(description, want) {
				t.Fatalf("UsageRecordResponse.latency_ms description = %q, want it to contain %q", description, want)
			}
		}
	})
}

// TestOpenAPIContract_FreeTextScopeIsStated pins what q searches (draft 010
// F8, owner decision D4 = expand), so the panel's placeholder is a promise the
// contract makes rather than one the gateway happens to keep.
func TestOpenAPIContract_FreeTextScopeIsStated(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{path: "/api/v1/usage/summary", want: usageQueryScope},
		{path: "/api/v1/usage/timeseries", want: usageQueryScope},
		{path: "/api/v1/usage/records", want: usageQueryScope},
		{path: "/api/v1/logs/requests", want: logQueryScope},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			_, schema := findUsageParam(t, tc.path, "get", "q")
			if schema.Description != tc.want {
				t.Fatalf("q description on %s = %q, want %q", tc.path, schema.Description, tc.want)
			}
		})
	}
}
