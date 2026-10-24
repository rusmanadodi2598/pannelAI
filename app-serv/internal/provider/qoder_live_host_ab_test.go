//go:build integration && live

// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_live_host_ab_test.go
// @for       The measurement draft 060 D2 waits on: how often each intl inference host answers the free model with a queue, probed on one account in one run.
// @uses      bytes, context, io, net/http, strings, testing, time, internal/registry.
// @reason    The connector moves every exchanged credential from the device host to the job host, and both its own comment and the upstream report say that rule was kept rather than measured. Deciding it needs one number nobody has: the queue rate per host on the same account at the same moment, which no unit test can produce and no third party owes. It runs on qfmodel because that model costs the account nothing, so the proof can be repeated without spending the owner's credit. This test reports; it does not decide.
//
//	PANNELAI_QODER_PAT='pt-…' \
//	  go test -tags=integration,live ./internal/provider/ -run QoderLiveQueueRatePerHost -v
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-17
package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// The probes are sequential and spaced because a caller firing both hosts at once
// would create the queue it then attributes to one of them.
const (
	qoderHostProbeRounds = 12
	qoderHostProbeGap    = 250 * time.Millisecond
	qoderHostProbeModel  = "qfmodel"
)

// qoderHostTally counts one host's answers by what they were, never by their status
// alone: a queue and a spent pool both arrive under codes that say neither.
type qoderHostTally struct {
	served   int
	queue    int
	backends int
	quota    int
	refused  int
	unsent   int
}

// TestQoderLiveQueueRatePerHost is the number D2 is waiting for. Whichever host leads
// alternates by round, because a vendor whose load drifts during the run would
// otherwise hand the second host the quieter half of it.
func TestQoderLiveQueueRatePerHost(t *testing.T) {
	connector, credential := qoderLiveConnector(t)
	model := requireLiveCatalog(t, connector, credential, qoderHostProbeModel)

	hosts := []string{qoderChatBaseIntlDevice, qoderChatBaseIntlJob}
	tally := map[string]*qoderHostTally{hosts[0]: {}, hosts[1]: {}}
	for round := 0; round < qoderHostProbeRounds; round++ {
		order := hosts
		if round%2 == 1 {
			order = []string{hosts[1], hosts[0]}
		}
		for _, host := range order {
			kind, detail := qoderProbeHost(t, connector, credential, model, host)
			tally[host].record(kind)
			if kind != "served" {
				t.Logf("%s round %d: %s (%s)", host, round, kind, detail)
			}
			time.Sleep(qoderHostProbeGap)
		}
	}

	sent := 0
	for _, host := range hosts {
		got := tally[host]
		t.Logf("%s: served=%d queue=%d backends=%d quota=%d refused=%d unsent=%d of %d",
			host, got.served, got.queue, got.backends, got.quota, got.refused, got.unsent,
			qoderHostProbeRounds)
		sent += qoderHostProbeRounds - got.unsent
	}
	if sent == 0 {
		t.Fatal("no probe reached either host, so the run measured nothing; check the token and the egress")
	}
}

// qoderProbeHost signs one agent call and sends it to the named host rather than to
// Endpoint(), because the host this measurement is about is exactly the choice
// Endpoint() makes.
func qoderProbeHost(t *testing.T, connector *Qoder, credential Credential, model registry.Model, host string) (string, string) {
	t.Helper()
	request := &Request{
		Model:  model,
		Wire:   "openai",
		Stream: true,
		Body: []byte(`{"model":"qoder/` + qoderHostProbeModel +
			`","messages":[{"role":"user","content":"Reply with exactly: PONG"}],"max_tokens":8}`),
		Credential: credential,
	}
	request.Provider = connector.entry
	if err := connector.TransformRequest(request); err != nil {
		return "unsent", "the body could not be built: " + err.Error()
	}

	url := host + qoderSigPathPrefix + qoderLiveChatPath
	outbound, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(request.Body))
	if err != nil {
		return "unsent", "the request could not be built: " + err.Error()
	}
	outbound.Header.Set("Content-Type", "application/json")
	outbound.Header.Set("Accept", "text/event-stream")
	if err := connector.ApplyAuth(outbound, credential); err != nil {
		return "unsent", "the request could not be signed: " + err.Error()
	}

	ctx, cancel := context.WithTimeout(outbound.Context(), qoderLiveTimeout)
	defer cancel()
	response, err := http.DefaultClient.Do(outbound.WithContext(ctx))
	if err != nil {
		return "unsent", "transport: " + err.Error()
	}
	defer func() {
		// reason: the answer is buffered below and read from the buffer.
		_ = response.Body.Close()
	}()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return "unsent", "the answer could not be read: " + readErr.Error()
	}
	if response.StatusCode != http.StatusOK {
		return qoderClassifyProbe(response.StatusCode, body, nil)
	}
	_, refusal := connector.OpenStream(io.NopCloser(bytes.NewReader(body)))
	return qoderClassifyProbe(response.StatusCode, body, refusal)
}

// qoderClassifyProbe names what one answer was. The queue is read first and from the
// body as well as the decoded refusal, because the vendor wraps it in a 403 at one
// path and states it in the envelope at the other, and those are the two shapes this
// draft spent its D1 on.
func qoderClassifyProbe(status int, body []byte, refusal *StreamFailure) (string, string) {
	lower := strings.ToLower(string(body))
	message := string(body)
	if refusal != nil {
		message = refusal.Message
		if refusal.Status == http.StatusTooManyRequests {
			return "queue", "envelope 429: " + truncateForLive(message, 120)
		}
	}
	switch {
	case strings.Contains(message, "10605"):
		return "queue", truncateForLive(message, 120)
	case strings.Contains(lower, "all backends failed"):
		return "backends", truncateForLive(message, 120)
	case strings.Contains(lower, "insufficient_quota") || strings.Contains(message, `"code":"112"`):
		return "quota", truncateForLive(message, 120)
	case refusal != nil:
		return "refused", http.StatusText(refusal.Status) + ": " + truncateForLive(message, 120)
	case status != http.StatusOK:
		return "refused", "http " + http.StatusText(status)
	case strings.Contains(lower, "choices") || strings.Contains(lower, "content"):
		return "served", "an answer with a delta"
	default:
		return "refused", "no answer and no refusal in " + truncateForLive(message, 120)
	}
}

func (h *qoderHostTally) record(kind string) {
	switch kind {
	case "served":
		h.served++
	case "queue":
		h.queue++
	case "backends":
		h.backends++
	case "quota":
		h.quota++
	case "unsent":
		h.unsent++
	default:
		h.refused++
	}
}
