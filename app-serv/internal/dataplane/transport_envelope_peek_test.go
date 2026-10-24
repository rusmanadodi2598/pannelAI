// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/transport_envelope_peek_test.go
// @for       The bound on a provider's stream-envelope peek when the vendor sends keepalives but no data event.
// @uses      context, io, sync, testing, time, internal/provider.
// @reason    The peek ran before the first-byte guard, so a keepalive-only stream could hold the request indefinitely; the test pins the budget and that the peek goroutine ends when the bound fires.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package dataplane

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

type blockingBody struct {
	release chan struct{}
	once    sync.Once
}

func (b *blockingBody) Read([]byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

func (b *blockingBody) Close() error {
	b.once.Do(func() { close(b.release) })
	return nil
}

type blockingEnvelope struct {
	done chan struct{}
}

func (b *blockingEnvelope) OpenStream(body io.ReadCloser) (io.ReadCloser, *provider.StreamFailure) {
	defer close(b.done)
	var one [1]byte
	_, _ = body.Read(one[:])
	return body, nil
}

func TestOpenEnvelopeWithinCutsAKeepaliveOnlyPeek(t *testing.T) {
	body := &blockingBody{release: make(chan struct{})}
	envelope := &blockingEnvelope{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := time.Now()
	opened, failure, err := openEnvelopeWithin(body, 20*time.Millisecond, cancel, envelope)
	if err == nil {
		t.Fatal("openEnvelopeWithin() = nil error, want the first-byte timeout")
	}
	if opened != nil || failure != nil {
		t.Fatalf("openEnvelopeWithin() = (%v, %v), want no body and no failure", opened, failure)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("peek took %v, want it cut near the 20ms budget", elapsed)
	}
	select {
	case <-envelope.done:
	case <-time.After(time.Second):
		t.Fatal("peek goroutine did not finish after the bound closed its body")
	}
	if ctx.Err() == nil {
		t.Fatal("the attempt context was not cancelled")
	}
}
