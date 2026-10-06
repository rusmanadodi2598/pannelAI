// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_envelope_quota_test.go
// @for       Which vendor refusals inside the SSE envelope park the account, and which only ask for another try.
// @uses      encoding/json, net/http, strings, testing.
// @reason    A refusal the gateway reads as spent stops being tried and parks the credential, while a refusal it reads as capacity is retried, so the two shapes the vendor uses for very different problems must not arrive at the same answer. It sends a real billing block as a numeric code, and a short free-model pool as a generic 429 with the complaint nested inside `details`. Measured 2026-09-28, the nested one recovered on the third attempt of the identical request.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// qoderEnvelopeFixture builds one wire frame with the escaping the vendor's shape
// demands: `body` is a JSON string that itself carries JSON, and `details` is a JSON
// string inside that. Hand-escaping either is how a test ends up asserting a shape
// nobody sends.
type qoderEnvelopeFixture struct {
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
	Value   int                 `json:"statusCodeValue"`
	Name    string              `json:"statusCode"`
}

func qoderRefusalFrame(t *testing.T, body string, status int, name string) string {
	t.Helper()
	frame, err := json.Marshal(qoderEnvelopeFixture{
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    body, Value: status, Name: name,
	})
	if err != nil {
		t.Fatalf("building the frame: %v", err)
	}
	return "data:" + string(frame) + "\n\n"
}

// qoderProviderErrorBody wraps an inner reason the way the model backend does: the
// refusal at the envelope is a fixed sentence, and the reason that explains it rides
// in `details` as another JSON string.
func qoderProviderErrorBody(t *testing.T, details string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"code": "provider_error", "message": "Error in upstream response",
		"type": "provider_error", "details": details,
	})
	if err != nil {
		t.Fatalf("building the body: %v", err)
	}
	return string(body)
}

func TestOpenStreamSeparatesABillingBlockFromAShortPool(t *testing.T) {
	cases := []struct {
		name        string
		frame       string
		wantQuota   bool
		wantStatus  int
		wantMessage string
	}{
		{
			name:       "code 112 with a pricing url is a billing block",
			frame:      qoderLiveQuotaBlock,
			wantQuota:  true,
			wantStatus: http.StatusForbidden, wantMessage: "pricing",
		},
		{
			name: "a numeric code arrives quoted and is still a billing block",
			frame: qoderRefusalFrame(t, `{"code":"110","message":"daily limit"}`,
				http.StatusForbidden, "FORBIDDEN"),
			wantQuota: true, wantStatus: http.StatusForbidden, wantMessage: "daily limit",
		},
		{
			name: "insufficient_quota nested in details keeps the retryable status",
			frame: qoderRefusalFrame(t, qoderProviderErrorBody(t,
				`{"error":{"message":"Workspace allocated quota exceeded, please increase your quota limit.",`+
					`"type":"insufficient_quota","code":"insufficient_quota"}}`),
				http.StatusTooManyRequests, "TOO_MANY_REQUESTS"),
			wantQuota: false, wantStatus: http.StatusTooManyRequests,
			wantMessage: "Workspace allocated quota exceeded",
		},
		{
			name: "the same reason named only by type is handled the same way",
			frame: qoderRefusalFrame(t, qoderProviderErrorBody(t,
				`{"error":{"message":"no quota left","type":"insufficient_quota","code":"rate_limit"}}`),
				http.StatusTooManyRequests, "TOO_MANY_REQUESTS"),
			wantQuota: false, wantStatus: http.StatusTooManyRequests, wantMessage: "no quota left",
		},
		{
			name: "another nested reason keeps the vendor's generic sentence",
			frame: qoderRefusalFrame(t, qoderProviderErrorBody(t,
				`{"error":{"message":"bad gateway","type":"upstream_error","code":"internal"}}`),
				http.StatusTooManyRequests, "TOO_MANY_REQUESTS"),
			wantQuota: false, wantStatus: http.StatusTooManyRequests,
			wantMessage: "Error in upstream response",
		},
		{
			name:        "details that are not json at all still refuse with the envelope status",
			frame:       qoderRefusalFrame(t, qoderProviderErrorBody(t, "not json"), http.StatusBadGateway, "BAD_GATEWAY"),
			wantQuota:   false,
			wantStatus:  http.StatusBadGateway,
			wantMessage: "Error in upstream response",
		},
		{
			name:        "no details stays a plain upstream failure",
			frame:       qoderRefusalFrame(t, qoderProviderErrorBody(t, ""), http.StatusTooManyRequests, "TOO_MANY_REQUESTS"),
			wantQuota:   false,
			wantStatus:  http.StatusTooManyRequests,
			wantMessage: "Error in upstream response",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, failure := newEnvelopeReader(t, tc.frame)
			if failure == nil {
				t.Fatal("the vendor's refusal was treated as success")
			}
			if failure.Quota != tc.wantQuota {
				t.Fatalf("quota = %v, want %v (%+v)", failure.Quota, tc.wantQuota, failure)
			}
			if failure.Status != tc.wantStatus {
				t.Fatalf("status = %d, want %d: a parked credential is never retried",
					failure.Status, tc.wantStatus)
			}
			if !strings.Contains(failure.Message, tc.wantMessage) {
				t.Fatalf("message = %q, want it to carry %q", failure.Message, tc.wantMessage)
			}
		})
	}
}
