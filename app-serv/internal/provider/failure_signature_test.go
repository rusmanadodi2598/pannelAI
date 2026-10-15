// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/failure_signature_test.go
// @for       The seeded signatures: which class each refusal a vendor is known to send lands in, and the reset and wait each one yields.
// @uses      net/http, testing, time, internal/domain.
// @reason    One 429 is a few minutes of load and another is a spent allowance with a date printed in its own body; classifying the second as the first is what re-burns a walk leg every cycle. These pin that pair against the exact strings the vendor sends, including the node an operator created in the panel.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"net/http"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// The two 429s TH Harbor sends, verbatim in the parts that decide the class.
const (
	thHeavyLoad = `{"error":{"message":"Token Harbor is under heavy load right now, ` +
		`so your requests have been slowed down for a few minutes."}}`
	thAllowanceSpent = `{"error":{"message":"You've used this period's free allowance. ` +
		`Your next period starts on 19 Oct 2026 at 10:48 UTC."}}`
	// thAllowanceReset is the instant the second one names, the park the old
	// five-minute ceiling was cutting short by four days.
	thAllowanceReset = "2026-10-19T10:48:00Z"
)

// TestClassify_RecognisesEachSeededSignature is the recognition layer: the same
// status, two bodies, two classes.
func TestClassify_RecognisesEachSeededSignature(t *testing.T) {
	reset, err := time.Parse(time.RFC3339, thAllowanceReset)
	if err != nil {
		t.Fatalf("parsing the fixture reset: %v", err)
	}
	cases := []struct {
		name      string
		failure   Failure
		wantClass domain.FailureClass
		wantID    string
		wantAt    time.Time
		wantRetry bool
	}{
		{
			name: "TH Harbor heavy load is a throttle",
			failure: Failure{
				Provider: "th-harbor", Host: "tokenharbor.ai", Status: http.StatusTooManyRequests,
				Body: []byte(thHeavyLoad),
			},
			wantClass: domain.FailureRateLimitTemporary,
			wantID:    "tokenharbor.heavy-load",
			wantRetry: true,
		},
		{
			name: "TH Harbor heavy load on a subdomain is the same throttle",
			failure: Failure{
				Provider: "th-harbor", Host: "api.tokenharbor.ai", Status: http.StatusTooManyRequests,
				Body: []byte(thHeavyLoad),
			},
			wantClass: domain.FailureRateLimitTemporary,
			wantID:    "tokenharbor.heavy-load",
			wantRetry: true,
		},
		{
			name: "TH Harbor spent allowance is a quota with a stated reset",
			failure: Failure{
				Provider: "th-harbor", Host: "tokenharbor.ai", Status: http.StatusTooManyRequests,
				Body: []byte(thAllowanceSpent),
			},
			wantClass: domain.FailureQuotaExhausted,
			wantID:    "tokenharbor.free-allowance",
			wantAt:    reset,
		},
		{
			name: "the Qoder envelope's quota flag is a spent account",
			failure: Failure{
				Provider: "qoder", Status: http.StatusForbidden, Quota: true,
				Body: []byte(`{"message":"Workspace allocated quota exceeded"}`),
			},
			wantClass: domain.FailureQuotaExhausted,
			wantID:    "qoder.envelope.quota",
		},
		{
			name: "the OpenCode gate refuses the gateway, not the key",
			failure: Failure{
				Provider: "opencode", Status: http.StatusBadGateway,
				Body: []byte(`OpenCode's free tier can only be used from within OpenCode`),
			},
			wantClass: domain.FailureGateBlocked,
			wantID:    "opencode.free-tier-gate",
		},
		{
			name:      "an unrecognised 429 is still the short throttle",
			failure:   Failure{Provider: "nobody", Status: http.StatusTooManyRequests, Body: []byte(`slow down`)},
			wantClass: domain.FailureRateLimitTemporary,
			wantRetry: true,
		},
		{
			name:      "a 5xx is a transient outage",
			failure:   Failure{Provider: "nobody", Status: http.StatusBadGateway, Body: []byte(`bad gateway`)},
			wantClass: domain.FailureServerTransient,
			wantRetry: true,
		},
		{
			name:      "a request-shaped 4xx stops the chain",
			failure:   Failure{Provider: "nobody", Status: http.StatusBadRequest, Body: []byte(`context length exceeded`)},
			wantClass: domain.FailureRequestInvalid,
		},
		{
			name:      "a 401 is a refused credential",
			failure:   Failure{Provider: "nobody", Status: http.StatusUnauthorized, Body: []byte(`invalid api key`)},
			wantClass: domain.FailureAuthInvalid,
		},
		{
			name:      "a 402 is a spent account",
			failure:   Failure{Provider: "nobody", Status: http.StatusPaymentRequired, Body: []byte(`insufficient credit`)},
			wantClass: domain.FailureBillingExhausted,
		},
		{
			name:      "a call that never reached the upstream is the target",
			failure:   Failure{Provider: "nobody", Status: 0},
			wantClass: domain.FailureTargetUnreachable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.failure)
			if got.Class != tc.wantClass {
				t.Fatalf("Classify(%s) class = %s, want %s", tc.name, got.Class, tc.wantClass)
			}
			if got.SignatureID != tc.wantID {
				t.Fatalf("Classify(%s) signature = %q, want %q", tc.name, got.SignatureID, tc.wantID)
			}
			if !tc.wantAt.IsZero() && !got.ResetAt.Equal(tc.wantAt) {
				t.Fatalf("Classify(%s) reset = %v, want %v", tc.name, got.ResetAt, tc.wantAt)
			}
			if got.Retryable() != tc.wantRetry {
				t.Fatalf("Classify(%s) retryable = %v, want %v", tc.name, got.Retryable(), tc.wantRetry)
			}
			if got.Class == "" {
				t.Fatalf("Classify(%s) answered no class at all", tc.name)
			}
		})
	}
}

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
