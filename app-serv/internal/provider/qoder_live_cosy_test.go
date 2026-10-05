//go:build integration && live

// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_live_cosy_test.go
// @for       The live proof that a request signed by this port is served by the
//
//	vendor, on the host its credential kind is expected to use.
//
// @uses      net/http, strings, testing.
// @reason    A signature can be self-consistent and still be wrong: the vendor
//
//	decides that, not a test that recomputes the same MD5. A model list is
//	the cheapest request that proves it, an empty body, no quota spent, and
//	a refusal on any header the vendor reads differently, and the second
//	case measures the host rule rather than trusting the reference's note
//	about it.
//
//	  PANNELAI_QODER_PAT='pt-…' \
//	    go test -tags=integration,live ./internal/provider/ -run QoderLive
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"net/http"
	"strings"
	"testing"
)

// qoderLiveSignedGet signs and sends one GET against a host's model list, and hands
// back the answer. The reference's own model list call is the shape: no body, JSON
// accepted, and identity encoding demanded, because a compressed answer trips the
// CDN's signature validation.
func qoderLiveSignedGet(t *testing.T, host string, account qoderLiveIdentity, jobToken string) (*http.Response, []byte) {
	t.Helper()
	requestURL := host + qoderLiveModelListPath
	signed, err := qoderCosy.headers(nil, requestURL, cosyIdentity{
		UserID: account.ID, AuthToken: jobToken, Name: account.Name,
		Email: account.Email, MachineID: newQoderID(),
	})
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	request.Header.Set("Accept", "application/json")
	// reason: gzip on this path triggers the CDN's signature validation, so the
	// official client and the reference both insist on identity encoding.
	request.Header.Set("Accept-Encoding", "identity")
	for name, values := range signed {
		request.Header.Set(name, values[0])
	}
	return qoderLiveClient(t, request)
}

// TestQoderLiveCOSYSignedModelListIsServed is the proof of the port: a request signed
// by this implementation is answered with the vendor's catalog. A wrong header
// casing, a reordered signature, or a mis-padded AES payload is a 401 or 403 here and
// nothing else, because the vendor validates all of it before it reads the path.
func TestQoderLiveCOSYSignedModelListIsServed(t *testing.T) {
	jobToken := qoderLiveExchange(t, qoderLivePAT(t))
	account := qoderLiveAccount(t, jobToken)

	response, body := qoderLiveSignedGet(t, qoderChatBaseIntlJob, account, jobToken)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("model list = HTTP %d: %s", response.StatusCode,
			truncateForLive(string(body), qoderLiveRefusalPreview))
	}
	if len(body) < 32 || !strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		t.Fatalf("the catalog is not the JSON the vendor publishes: %s",
			truncateForLive(string(body), 200))
	}
	t.Logf("signed model list served from the job host: %d bytes", len(body))
}

// TestQoderLiveJobTokenOnTheDeviceHost measures the rule the host split exists for.
// The reference records that the device host answers a job token with 403 "Login
// expired"; this asserts nothing about the status, it reports what the vendor does
// today so a change upstream is written down rather than quietly inherited.
func TestQoderLiveJobTokenOnTheDeviceHost(t *testing.T) {
	jobToken := qoderLiveExchange(t, qoderLivePAT(t))
	account := qoderLiveAccount(t, jobToken)

	response, body := qoderLiveSignedGet(t, qoderChatBaseIntlDevice, account, jobToken)
	t.Logf("device host answer to a job token: HTTP %d, %s", response.StatusCode,
		truncateForLive(string(body), 200))
	if response.StatusCode == http.StatusOK {
		t.Log("the device host serves a job token today; the reference's host rule may be obsolete for reads")
	}
}
