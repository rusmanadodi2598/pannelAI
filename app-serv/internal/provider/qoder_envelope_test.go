// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_envelope_test.go
// @for       The envelope unwrap: what a refusal becomes, and what a client reads
//
//	when the provider's answer arrives wrapped.
//
// @uses      io, net/http, strings, testing.
// @reason    The vendor hides the real status inside each frame, so a spent account
//
//	arrives as HTTP 200 (draft 036 §5.1) and the only thing between that
//	and a billed success is this unwrap. The refusal case uses the exact
//	bytes the live service sent, captured 2026-09-27.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// qoderLiveQuotaBlock is the real first frame the vendor returned for a Personal
// Access Token whose account is spent: HTTP 200 outside, 403 and code 112 inside.
const qoderLiveQuotaBlock = `data:{"headers":{"Content-Type":["application/json"]},"body":"{\"code\":\"112\",\"message\":\"{\\\"pricingUrl\\\":\\\"https://qoder.com/pricing?client=qoder\\\"}\"}","statusCodeValue":403,"statusCode":"FORBIDDEN"}` + "\n\n"

// newEnvelopeReader serves a scripted body through the connector's seam.
func newEnvelopeReader(t *testing.T, body string) (io.ReadCloser, *StreamFailure) {
	t.Helper()
	connector := newQoderTestConnector(t,
		qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), nil)
	return connector.OpenStream(io.NopCloser(strings.NewReader(body)))
}

func readAll(t *testing.T, reader io.ReadCloser) string {
	t.Helper()
	out, err := io.ReadAll(reader)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("reading the unwrapped stream: %v", err)
	}
	return string(out)
}

// TestOpenStreamReportsTheVendorsQuotaBlockAsFailure pins the case that matters most
// for accounting: a spent account must not be piped to the client as an answer, and
// it must be recognisable as quota so the account is parked rather than backed off.
func TestOpenStreamReportsTheVendorsQuotaBlockAsFailure(t *testing.T) {
	reader, failure := newEnvelopeReader(t, qoderLiveQuotaBlock)
	if reader != nil {
		t.Fatal("a refusal still produced a body to pipe")
	}
	if failure == nil {
		t.Fatal("the vendor's 403 inside a 200 was treated as success")
	}
	if failure.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want the provider's 403", failure.Status)
	}
	if !failure.Quota {
		t.Fatalf("quota = false, want code 112 to read as a spent account: %+v", failure)
	}
	if !strings.Contains(failure.Message, "pricing") {
		t.Fatalf("message = %q, want the vendor's own reason", failure.Message)
	}
}

// TestOpenStreamUnwrapsEveryFrameAndReplaysThePeek is the happy path: the client reads
// plain OpenAI chunks, no envelope survives, and the frame the peek consumed is not
// lost.
func TestOpenStreamUnwrapsEveryFrameAndReplaysThePeek(t *testing.T) {
	stream := `data:{"statusCodeValue":200,"body":"{\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"}}]}"}

` + `data:{"statusCodeValue":200,"body":"{\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}"}

` + `data:{"statusCodeValue":200,"body":"{\"id\":\"c1\",\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}"}

` + "data:[DONE]\n\n"

	reader, failure := newEnvelopeReader(t, stream)
	if failure != nil {
		t.Fatalf("a successful stream was reported as a failure: %+v", failure)
	}
	got := readAll(t, reader)

	if strings.Contains(got, "statusCodeValue") {
		t.Fatalf("an envelope survived the unwrap:\n%s", got)
	}
	for _, want := range []string{`"content":"Hel"`, `"content":"lo"`, `"finish_reason":"stop"`, `"prompt_tokens":5`, "data: [DONE]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("unwrapped stream is missing %s:\n%s", want, got)
		}
	}
	if !strings.HasPrefix(got, "data: {") {
		t.Fatalf("the first frame was not replayed:\n%s", got)
	}
}

// TestOpenStreamKeepsAKeealiveAndNoDataEventQuietly pins that a frame carrying nothing
// is skipped rather than turned into an empty chunk a client would have to explain.
func TestOpenStreamKeepsAKeealiveAndNoDataEventQuietly(t *testing.T) {
	stream := ": keepalive\n\n" +
		`data:{"statusCodeValue":200,"body":"{\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}"}

` + "data:[DONE]\n\n"

	reader, failure := newEnvelopeReader(t, stream)
	if failure != nil {
		t.Fatalf("failure = %+v, want the stream to open", failure)
	}
	got := readAll(t, reader)
	if strings.Contains(got, "keepalive") {
		t.Fatalf("a comment event reached the client:\n%s", got)
	}
	if strings.Count(got, "data:") != 2 {
		t.Fatalf("frames = %d, want the content frame and the terminal marker:\n%s",
			strings.Count(got, "data:"), got)
	}
}

// TestOpenStreamPassesThroughAnUnwrappedProvider pins the tolerance: a provider that
// answers as plain OpenAI (or changes shape later) is not met by a gateway that drops
// the frames it cannot parse.
func TestOpenStreamPassesThroughAnUnwrappedProvider(t *testing.T) {
	stream := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"plain\"}}]}\n\ndata: [DONE]\n\n"

	reader, failure := newEnvelopeReader(t, stream)
	if failure != nil {
		t.Fatalf("failure = %+v, want the body piped through", failure)
	}
	got := readAll(t, reader)
	if !strings.Contains(got, `"plain"`) || !strings.Contains(got, "[DONE]") {
		t.Fatalf("an unwrapped stream was altered:\n%s", got)
	}
}

// TestOpenStreamClosesOnARefusalAfterTheFirstFrame pins the late refusal: the status
// is already with the client, so the answer ends with the provider's reason as content
// and a terminal marker rather than a silent truncation.
func TestOpenStreamClosesOnARefusalAfterTheFirstFrame(t *testing.T) {
	stream := `data:{"statusCodeValue":200,"body":"{\"choices\":[{\"index\":0,\"delta\":{\"content\":\"start\"}}]}"}

` + `data:{"statusCodeValue":429,"body":"{\"code\":\"10605\",\"message\":\"queue throttle\"}"}

`

	reader, failure := newEnvelopeReader(t, stream)
	if failure != nil {
		t.Fatalf("failure = %+v, want the stream to open on its first frame", failure)
	}
	got := readAll(t, reader)
	if !strings.Contains(got, `"start"`) {
		t.Fatalf("the frames before the refusal were dropped:\n%s", got)
	}
	if !strings.Contains(got, "qoder error 429") {
		t.Fatalf("the late refusal is not visible in the answer:\n%s", got)
	}
	if !strings.HasSuffix(got, "data: [DONE]\n\n") {
		t.Fatalf("the stream did not end with the terminal marker:\n%s", got)
	}
}

// TestOpenStreamRefusesNoBody pins the seam's own guard: a nil body is the provider's
// fault reported as a failure, not a panic in the reader.
func TestOpenStreamRefusesNoBody(t *testing.T) {
	connector := newQoderTestConnector(t,
		qoderEntry("qoder", "https://openapi.qoder.sh", qoderChatURLIntl), nil)
	_, failure := connector.OpenStream(nil)
	if failure == nil || failure.Status != http.StatusBadGateway {
		t.Fatalf("failure = %+v, want a bad-gateway refusal", failure)
	}
}
