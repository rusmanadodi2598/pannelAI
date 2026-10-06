// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_fusion_bound_test.go
// @for       The fusion fan-out's bound: a large panel is served in waves, not as one goroutine per member against the shared connection pool.
// @uses      context, fmt, net/http, net/http/httptest, sync, testing, time, internal/domain.
// @reason    A combo may name dozens of members and each relay can walk credentials and retry, so an unbounded fan-out starves every other request on the process. The peak in-flight panel call is measured.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-03
package dataplane

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// concurrencyProbe records the peak number of simultaneous requests.
type concurrencyProbe struct {
	mu       sync.Mutex
	inFlight int
	peak     int
}

func (p *concurrencyProbe) enter() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inFlight++
	if p.inFlight > p.peak {
		p.peak = p.inFlight
	}
}

func (p *concurrencyProbe) leave() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inFlight--
}

func (p *concurrencyProbe) highest() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.peak
}

// TestRelay_FusionBoundsTheFanOut drives an eight-member panel and pins that no
// more than four members are in flight at once (the value fusionFanOutLimit
// states), while the fan-out stays parallel: a serial panel would peak at one.
func TestRelay_FusionBoundsTheFanOut(t *testing.T) {
	const wantLimit = 4
	probe := &concurrencyProbe{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.enter()
		defer probe.leave()
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-x","object":"chat.completion","created":1,"model":"m",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"panel answer"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(server.Close)

	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	refs := []string{"alpha/judge"}
	for index := 1; index <= 8; index++ {
		refs = append(refs, fmt.Sprintf("alpha/m%d", index))
	}
	combo := fusionRow("panel", refs[0], refs[1:]...)
	engine := newEngineWith(t, fusionProviders(server.URL, "alpha"), repo,
		map[string]domain.Combo{"panel": combo}, nil)

	if _, err := engine.Relay(context.Background(), fusionRequest("panel", false, false), nil); err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	peak := probe.highest()
	if peak > wantLimit {
		t.Fatalf("panel concurrency peaked at %d, want at most %d", peak, wantLimit)
	}
	if peak < 2 {
		t.Fatalf("panel concurrency peaked at %d, want the fan-out to stay parallel", peak)
	}
}
