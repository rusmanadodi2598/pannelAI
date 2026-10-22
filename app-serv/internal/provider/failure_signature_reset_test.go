// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/failure_signature_reset_test.go
// @for       The reset and wait a classified refusal yields, plus the Google quota markers.
// @uses      net/http, testing, time, internal/domain.
// @reason    The recognition layer answers a class, but the park length decides whether a working credential comes back in five minutes or a day. These cases pin which source wins (body date over header), that an operator-created node is recognised by wording alone, and that Google's bare exhausted status stays a short throttle.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-22
package provider

import (
	"net/http"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestClassify_AllowanceResetIsReadFromTheBodyNotTheHeader is C2: the park has to
// end on the date the vendor printed, not on a header it did not send.
func TestClassify_AllowanceResetIsReadFromTheBodyNotTheHeader(t *testing.T) {
	got := Classify(Failure{
		Provider: "th-harbor", Host: "tokenharbor.ai", Status: http.StatusTooManyRequests,
		Body:   []byte(thAllowanceSpent),
		Header: http.Header{"Retry-After": []string{"300"}},
	})
	if got.ResetSource != domain.ResetSourceBody {
		t.Fatalf("reset source = %q, want %q: the body's date outranks the header", got.ResetSource, domain.ResetSourceBody)
	}
	if got.ResetAt.IsZero() {
		t.Fatal("reset = zero, want the date out of the body")
	}
	if got.RetryAfter != 0 {
		t.Fatalf("retry-after = %v, want it unused once the body named a date", got.RetryAfter)
	}
}

// TestClassify_HonorsRetryAfterWhenNoResetIsStated is the other half: a throttle
// that names no date still gets the vendor's own wait, recorded as a header.
func TestClassify_HonorsRetryAfterWhenNoResetIsStated(t *testing.T) {
	got := Classify(Failure{
		Provider: "th-harbor", Host: "tokenharbor.ai", Status: http.StatusTooManyRequests,
		Body:   []byte(thHeavyLoad),
		Header: http.Header{"Retry-After": []string{"120"}},
	})
	if got.RetryAfter != 2*time.Minute {
		t.Fatalf("retry-after = %v, want the header's 120s", got.RetryAfter)
	}
	if got.ResetSource != domain.ResetSourceHeader {
		t.Fatalf("reset source = %q, want %q", got.ResetSource, domain.ResetSourceHeader)
	}
	if !got.ResetAt.IsZero() {
		t.Fatalf("reset = %v, want no absolute reset for a header wait", got.ResetAt)
	}
}

// TestClassify_THarborIsRecognisedOnAnyHost is the operator-created node's case:
// its id and its host are chosen in the panel, so the wording is all there is.
func TestClassify_THarborIsRecognisedOnAnyHost(t *testing.T) {
	got := Classify(Failure{
		Provider: "openai-compatible-harbor", Host: "127.0.0.1", Status: http.StatusTooManyRequests,
		Body: []byte(thAllowanceSpent),
	})
	if got.Class != domain.FailureQuotaExhausted || got.SignatureID != "tokenharbor.free-allowance" {
		t.Fatalf("Classify() = %s/%s, want the allowance signature on any host", got.Class, got.SignatureID)
	}
}

// TestClassify_GoogleQuotaMarkersSeparateThrottleFromSpent is the parking rule for
// the Gemini families: Google reports both a full pool and a spent allowance under a
// 429 whose status alone cannot tell them apart, and the difference is what the
// operator pays for. A spent allowance belongs on the long ladder; a pool that frees
// in seconds must not park a working credential for a day.
func TestClassify_GoogleQuotaMarkersSeparateThrottleFromSpent(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		host      string
		wantClass domain.FailureClass
		wantID    string
	}{
		{
			name:      "a named rate-limit reason is spent quota",
			body:      `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED"}]}}`,
			wantClass: domain.FailureQuotaExhausted, wantID: "google.quota-exhausted",
		},
		{
			name:      "the quota bucket spent wording is spent quota",
			body:      `{"error":{"code":403,"message":"Individual quota reached for the current model."}}`,
			wantClass: domain.FailureQuotaExhausted, wantID: "google.quota-exhausted",
		},
		{
			// The reference keeps this wording off the quota ladder on purpose, and
			// this is the line a careless signature would erase.
			name:      "bare exhausted status stays a short throttle",
			body:      `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"Resource has been exhausted"}}`,
			wantClass: domain.FailureRateLimitTemporary, wantID: "",
		},
		{
			name: "the markers do not travel to another vendor",
			body: `{"error":{"reason":"QUOTA_EXHAUSTED"}}`, host: "api.example.com",
			wantClass: domain.FailureRateLimitTemporary, wantID: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host := tc.host
			if host == "" {
				host = "daily-cloudcode-pa.googleapis.com"
			}
			failure := Failure{
				Provider: "gemini-cli", Host: host, Status: http.StatusTooManyRequests,
				Body: []byte(tc.body),
			}
			verdict := Classify(failure)
			if verdict.Class != tc.wantClass {
				t.Fatalf("class = %q, want %q (body=%s)", verdict.Class, tc.wantClass, tc.body)
			}
			if verdict.SignatureID != tc.wantID {
				t.Fatalf("signature = %q, want %q", verdict.SignatureID, tc.wantID)
			}
		})
	}
}
