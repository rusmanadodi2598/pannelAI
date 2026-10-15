// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_sessions.go
// @for       Serialization of the outbound calls that share one upstream session.
// @uses      context, io, strings, sync, time, internal/provider.
// @reason    The OpenCode free tier throttles per session and every request the gateway sends presents one session per (client, endpoint) pair, so parallel requests stack upstream and are released together. A connector that knows its session identity declares it through provider.SessionSerializer and the transport runs one call at a time for that key. The gate lives here rather than in the connector because only the transport owns the call's whole lifetime, which ends when the answer body is closed rather than when the headers arrive.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// sessionQueue holds one FIFO gate per upstream session. Gates are kept for the
// life of the process: a key is a configured (client, endpoint) pair, so the
// map is bounded by the registry and the endpoint table rather than by traffic.
type sessionQueue struct {
	mu    sync.Mutex
	gates map[string]chan struct{}
}

func newSessionQueue() *sessionQueue {
	return &sessionQueue{gates: map[string]chan struct{}{}}
}

// acquire takes the session's slot, waiting for the holder to release it. A
// blocked send queues in FIFO order (Go channels are FIFO), so the requests
// waiting on one session are served in the order they arrived rather than in
// whatever order the scheduler wakes them. A canceled waiter leaves the queue
// without taking the slot, which is what keeps a client that hung up while
// queued from blocking the ones behind it.
func (q *sessionQueue) acquire(ctx context.Context, session string) (func(), error) {
	q.mu.Lock()
	gate, ok := q.gates[session]
	if !ok {
		gate = make(chan struct{}, 1)
		q.gates[session] = gate
	}
	q.mu.Unlock()

	select {
	case gate <- struct{}{}:
		// A canceled caller may win the race against the slot coming free; it
		// must not hold the slot for an attempt that is already dead.
		if err := ctx.Err(); err != nil {
			<-gate
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-gate }) }, nil
}

// sessionKeyFor answers the session one call belongs to, or "" when the
// connector declares no session identity (every provider but OpenCode Free).
func sessionKeyFor(plugin provider.Plugin, call Call) string {
	serializer, ok := plugin.(provider.SessionSerializer)
	if !ok {
		return ""
	}
	return strings.TrimSpace(serializer.SessionKey(RequestFor(call)))
}

// acquireSession takes the session slot a call must hold, and hands the queue
// wait back to the walk's budget: a request that waited locally did not spend
// upstream time, so its attempt still gets the bound the walk would have given
// it (see RequestBudget.pause). The returned release is a no-op when the
// connector declares no session.
func (t *Transport) acquireSession(ctx context.Context, plugin provider.Plugin, call Call) (func(), error) {
	key := sessionKeyFor(plugin, call)
	if key == "" || t.sessions == nil {
		return func() {}, nil
	}
	started := time.Now()
	release, err := t.sessions.acquire(ctx, key)
	if err != nil {
		return nil, err
	}
	if call.Budget != nil {
		call.Budget.pause(time.Since(started))
	}
	return release, nil
}

// sessionBody hands the session slot back when the caller closes the answer.
// The slot is held until then because the upstream request is in flight until
// the body is read to its end, and releasing at the headers would let the next
// request stack onto the same session while this one is still being served.
type sessionBody struct {
	io.ReadCloser
	release func()
}

func (b *sessionBody) Close() error {
	err := b.ReadCloser.Close()
	b.release()
	return err
}
