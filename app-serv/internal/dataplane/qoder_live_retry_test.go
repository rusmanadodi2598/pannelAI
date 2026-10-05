//go:build integration && live

// Package dataplane implements the request-path data plane of app-serv.
//
// @file      internal/dataplane/qoder_live_retry_test.go
// @for       One real Qoder free-model call driven through the transport's own retry
//
//	loop, to prove the gateway rides out the vendor's transient capacity refusal
//	instead of handing it back as a dead model.
//
// @uses      context, io, net/http, os, strings, sync/atomic, testing, time,
//
//	internal/provider, internal/registry.
//
// @reason    The unit retry proof (transport_envelope_retry_test.go) drives a fake
//
//	that refuses on schedule. This asks the vendor: Qoder's free model answers the
//	signed request with "all backends failed" / "quota exceeded" and serves the
//	identical request seconds later (draft 036 §9.2), so a single attempt can read
//	as a broken provider. The claim worth proving live is not that the vendor never
//	refuses, it does, but that the gateway retried a refusal the client never saw
//	an answer for, rather than giving up on the first frame. It tolerates the vendor
//	genuinely running out of its lent capacity; it does not tolerate a one-attempt
//	give-up, which is the regression this whole change is about.
//
//	  PANNELAI_QODER_PAT='pt-…' \
//	    go test -tags=integration,live ./internal/dataplane/ -run QoderLiveRetry
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-28
package dataplane

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// countingTransport is an http.RoundTripper that records how many outbound chat
// calls the transport made, delegating to a real client for the wire. The exchange,
// identity, and catalogue reads ride the connector's own client, so this counts
// exactly the agent-call attempts the retry loop produced.
type countingTransport struct {
	base  http.RoundTripper
	calls atomic.Int64
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return c.base.RoundTrip(req)
}

// TestQoderLiveRetryServesTheFreeModel asks Qwen3.8-Flash through the whole gateway,
// shape, sign, and the floored seconds-scale retry, and asserts the retry loop is
// the thing that reaches an answer, not a single lucky shot.
func TestQoderLiveRetryServesTheFreeModel(t *testing.T) {
	pat := strings.TrimSpace(os.Getenv("PANNELAI_QODER_PAT"))
	if pat == "" {
		t.Fatal("PANNELAI_QODER_PAT must be set for the live retry proof")
	}

	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	entry, ok := index.Provider("qoder")
	if !ok {
		t.Fatal("the embedded registry carries no qoder entry")
	}
	if got := entry.Transport.Retry.Attempts(http.StatusTooManyRequests); got < 2 {
		t.Fatalf("the qoder entry retries a 429 %d times, want more than one attempt", got)
	}

	connector, err := provider.NewQoder(entry, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	connectors, err := provider.NewConnectors(provider.DefaultFactory, connector)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	counter := &countingTransport{base: http.DefaultTransport}
	transport, err := NewTransport(TransportDeps{
		Connectors: connectors,
		Client:     &http.Client{Transport: counter, Timeout: 150 * time.Second},
	})
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}

	credential := provider.StaticKey("ep_live", "k_live", pat)
	call := Call{
		Provider: entry, Model: registry.Model{ID: "qfmodel"}, Wire: "openai", Stream: true,
		Body: []byte(`{"model":"qoder/qfmodel","messages":[{"role":"user",` +
			`"content":"Reply with exactly: PONG"}],"max_tokens":48}`),
		Credential: credential,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	upstream, doErr := transport.Do(ctx, call)
	attempts := counter.calls.Load()

	if doErr != nil {
		// The vendor can genuinely run its lent free capacity out for the whole
		// ladder window. That is its capacity to lend, not a gateway fault, but a
		// gateway that gave up on the first frame is exactly the bug. So a failure
		// is only acceptable when the retry loop actually repeated a refusal the
		// client never saw an answer for.
		if attempts < 2 {
			t.Fatalf("the transport made %d chat attempt(s) then failed: %v, a retryable "+
				"free-pool refusal must be repeated, not given up on", attempts, doErr)
		}
		t.Logf("the vendor's free pool stayed out across %d attempts: %v", attempts, doErr)
		return
	}

	defer func() {
		// reason: the answer is read in full below.
		_ = upstream.Body.Close()
	}()
	answer, err := io.ReadAll(io.LimitReader(upstream.Body, 1<<20))
	if err != nil {
		t.Fatalf("reading the served stream: %v", err)
	}
	if !strings.Contains(string(answer), "chat.completion.chunk") {
		t.Fatalf("no OpenAI chunk reached the client after %d attempts: %s",
			attempts, headOf(string(answer), 300))
	}
	if strings.Contains(string(answer), "statusCodeValue") {
		t.Fatalf("an envelope reached the client: %s", headOf(string(answer), 300))
	}
	t.Logf("Qwen3.8-Flash served through the gateway in %d attempt(s): %d bytes of unwrapped stream",
		attempts, len(answer))
}

// headOf shortens a live answer for a failure message, so a rejected stream is
// reported without dumping megabytes into the test log.
func headOf(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
