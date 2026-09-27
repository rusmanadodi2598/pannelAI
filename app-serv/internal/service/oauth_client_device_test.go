// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/oauth_client_device_test.go
// @for       The device-token poll client's wire behaviour and the upstream's
//
//	expiry spellings (draft 036 slice A).
//
// @uses      testing, encoding/json, net/http, net/http/httptest, strings,
//
//	time.
//
// @reason    The poll is the one place the vendor's tolerances live: 202 and
//
//	404 both mean "not yet", a refusal may carry a message, and the
//	expiry arrives as a number, a numeric string, or a date. Those
//	rules cannot be proven from the flow service, which only ever sees
//	the decoded answer, so an httptest upstream asserts them here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-27
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// devicePollFixture is one scripted upstream answer, the client pointed at it,
// and the request that produced it.
type devicePollFixture struct {
	client *OAuthHTTPClient
	url    string
	seen   *devicePollRequest
}

// devicePollRequest is what one poll attempt put on the wire.
type devicePollRequest struct {
	method string
	path   string
	query  string
	ua     string
	accept string
}

// newDevicePollServer serves one scripted answer and records the request that
// produced it.
func newDevicePollServer(t *testing.T, status int, body string) devicePollFixture {
	t.Helper()
	seen := &devicePollRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = devicePollRequest{
			method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
			ua: r.Header.Get("User-Agent"), accept: r.Header.Get("Accept"),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return devicePollFixture{client: NewOAuthHTTPClient(server.Client()), url: server.URL + "/api/v1/deviceToken/poll", seen: seen}
}

// poll runs one attempt with the round a start would have staged.
func (f devicePollFixture) poll() (DeviceTokenResponse, bool, error) {
	return f.client.DevicePoll(context.Background(), f.url, "nonce-1", "verifier-1")
}

// TestDevicePoll_AsksTheVendorTheWayTheVendorExpects pins the request shape: a
// GET carrying exactly the parameters the device endpoint reads, under the CLI
// user agent the CDN serves.
func TestDevicePoll_AsksTheVendorTheWayTheVendorExpects(t *testing.T) {
	fixture := newDevicePollServer(t, http.StatusAccepted, "")

	token, pending, err := fixture.poll()
	if err != nil {
		t.Fatalf("DevicePoll: %v", err)
	}
	if !pending {
		t.Fatalf("pending = false, want a 202 to keep the modal waiting")
	}
	if token.AccessToken != "" {
		t.Fatalf("a pending poll returned token material: %+v", token)
	}
	if fixture.seen.method != http.MethodGet {
		t.Fatalf("method = %q, want GET", fixture.seen.method)
	}
	if fixture.seen.path != "/api/v1/deviceToken/poll" {
		t.Fatalf("path = %q", fixture.seen.path)
	}
	if fixture.seen.ua != devicePollUserAgent || fixture.seen.accept != "application/json" {
		t.Fatalf("headers = %q / %q, want the CLI user agent and JSON accept", fixture.seen.ua, fixture.seen.accept)
	}
	if !strings.Contains(fixture.seen.query, "nonce=nonce-1") || !strings.Contains(fixture.seen.query, "verifier=verifier-1") {
		t.Fatalf("query = %q, want the nonce under `nonce` and the verifier under `verifier`", fixture.seen.query)
	}
	if !strings.Contains(fixture.seen.query, "challenge_method=S256") {
		t.Fatalf("query = %q, want the S256 challenge method", fixture.seen.query)
	}
	if strings.Contains(fixture.seen.query, "machine_id") {
		t.Fatalf("query = %q, want no machine_id: the poll endpoint never reads it", fixture.seen.query)
	}
}

// TestDevicePoll_TreatsAFirstPollMissAsPending pins the 404 answer: the vendor
// has not registered the round yet, which is waiting, not failure.
func TestDevicePoll_TreatsAFirstPollMissAsPending(t *testing.T) {
	fixture := newDevicePollServer(t, http.StatusNotFound, `{"message":"not found"}`)
	if _, pending, err := fixture.poll(); err != nil || !pending {
		t.Fatalf("pending = %v, err = %v, want a 404 to keep waiting", pending, err)
	}
}

// TestDevicePoll_ReadsTheIssuedToken pins the success body: the token rides as
// `token`, the identity as `user_id`, and the expiry as an absolute instant.
func TestDevicePoll_ReadsTheIssuedToken(t *testing.T) {
	fixture := newDevicePollServer(t, http.StatusOK,
		`{"token":"dt-abc","refresh_token":"rt-abc","user_id":"user-7","expires_at":1781594470000}`)

	token, pending, err := fixture.poll()
	if err != nil || pending {
		t.Fatalf("pending = %v, err = %v", pending, err)
	}
	if token.AccessToken != "dt-abc" || token.RefreshToken != "rt-abc" || token.UserID != "user-7" {
		t.Fatalf("token = %+v", token)
	}
	if !token.ExpiresAt.Equal(time.UnixMilli(1781594470000)) {
		t.Fatalf("expiry = %v, want the upstream instant", token.ExpiresAt)
	}
}

// TestDevicePoll_Refusals pins the answers that end a modal: a refusal that
// quotes the vendor's own reason, and a 200 that carries no credential — storing
// nothing beats storing an empty token.
func TestDevicePoll_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"a refusal naming its reason", http.StatusForbidden, `{"message":"nonce expired"}`, "nonce expired"},
		{"a refusal without a body", http.StatusBadGateway, "", "HTTP 502"},
		{"an unparseable answer", http.StatusOK, "not json", "could not be decoded"},
		{"a success without a token", http.StatusOK, `{"user_id":"user-7"}`, "without a token"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newDevicePollServer(t, testCase.status, testCase.body)
			_, pending, err := fixture.poll()
			if err == nil || pending {
				t.Fatalf("pending = %v, err = %v, want a terminal failure", pending, err)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %q, want it to mention %q", err, testCase.want)
			}
		})
	}
}

// TestDevicePoll_RefusesAnIncompleteRound pins the client's own guard: a poll
// without its nonce or verifier cannot be answered by the vendor, so it is never
// sent.
func TestDevicePoll_RefusesAnIncompleteRound(t *testing.T) {
	fixture := newDevicePollServer(t, http.StatusOK, `{"token":"dt-should-not-be-read"}`)

	for _, args := range [][2]string{{"", "verifier-1"}, {"nonce-1", ""}} {
		if _, _, err := fixture.client.DevicePoll(context.Background(), fixture.url, args[0], args[1]); err == nil {
			t.Fatalf("DevicePoll(%q, %q) accepted an incomplete round", args[0], args[1])
		}
	}
	if fixture.seen.method != "" {
		t.Fatalf("an incomplete round still reached the upstream: %s %s", fixture.seen.method, fixture.seen.query)
	}
}

// TestParseDeviceExpiry pins the reference's parsing order: a numeric ms epoch
// (number or numeric string) before any date parsing, then RFC3339, then
// expires_in seconds, then the thirty-day default.
func TestParseDeviceExpiry(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		expiresAt  string // raw JSON: quoted means a string, unquoted a number
		expiresIn  *int
		want       time.Time
		wantWithin time.Duration // a non-zero tolerance means "about now plus this"
	}{
		{name: "a numeric ms epoch survives as-is", expiresAt: "1781594470000", want: time.UnixMilli(1781594470000)},
		{name: "a numeric string ms epoch survives as-is", expiresAt: `"1781594470000"`, want: time.UnixMilli(1781594470000)},
		{name: "an RFC3339 string parses", expiresAt: `"2026-12-16T07:15:04Z"`, want: time.Date(2026, 12, 16, 7, 15, 4, 0, time.UTC)},
		{name: "a short numeric string is an epoch, not a year", expiresAt: `"2026"`, want: time.UnixMilli(2026)},
		{name: "expires_in counts from now", expiresAt: "null", expiresIn: intPtr(90), want: now.Add(90 * time.Second), wantWithin: time.Second},
		{name: "everything missing falls back to thirty days", expiresAt: "null", want: now.Add(deviceExpiryDefault), wantWithin: time.Second},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := parseDeviceExpiry(json.RawMessage(testCase.expiresAt), testCase.expiresIn, now)
			if testCase.wantWithin > 0 {
				if diff := signedDiff(got.Sub(testCase.want)); diff > testCase.wantWithin {
					t.Fatalf("expiry = %v, want within %v of %v", got, testCase.wantWithin, testCase.want)
				}
				return
			}
			if !got.Equal(testCase.want) {
				t.Fatalf("expiry = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestFloorDeviceExpiry pins the wrapper's floor: a token the upstream claims is
// already dead still stores a day of life, and a healthy expiry is left alone.
func TestFloorDeviceExpiry(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	floor := now.Add(deviceExpiryFloor)

	if got := floorDeviceExpiry(now.Add(-time.Hour), now); !got.Equal(floor) {
		t.Fatalf("a past expiry floored to %v, want %v", got, floor)
	}
	if got := floorDeviceExpiry(now.Add(time.Minute), now); !got.Equal(floor) {
		t.Fatalf("a near expiry floored to %v, want %v", got, floor)
	}
	if got := floorDeviceExpiry(now.Add(48*time.Hour), now); !got.Equal(now.Add(48 * time.Hour)) {
		t.Fatalf("a healthy expiry was changed to %v", got)
	}
}

func signedDiff(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
