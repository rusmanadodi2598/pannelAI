// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_sessions.go
// @for       Serialization of the outbound calls that share one upstream session.
// @uses      context, io, strings, sync, time, internal/provider.
// @reason    Every request the gateway sends on the OpenCode free lane presents one session per (client, endpoint) pair, so parallel calls stack behind that identity and are released together: a connector that knows its session declares it through provider.SessionSerializer and the transport runs one call at a time for that key. Measured against the live lane on 2026-10-15, the 429 that lane answers with is keyed on the egress address rather than the session (SPEC-API-001 §7.11), so this gate bounds concurrency and is not the quota's unit. The gate lives here rather than in the connector because only the transport owns the call's whole lifetime, which ends when the answer body is closed rather than when the headers arrive.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// sessionWaitWarnBounds the local queue wait that earns a log line. A healthy lane
// hands the slot over in microseconds; anything past this is a call stacked behind
// an answer that has not finished, which no upstream error can explain.
const sessionWaitWarnBounds = 2 * time.Second

// sessionQueue holds one FIFO gate per upstream session. Gates are kept for the
// life of the process: a key is a configured (client, endpoint) pair, so the
// map is bounded by the registry and the endpoint table rather than by traffic.
type sessionQueue struct {
	mu      sync.Mutex
	gates   map[string]chan struct{}
	waiters int
	logger  *slog.Logger
}

func newSessionQueue(logger *slog.Logger) *sessionQueue {
	return &sessionQueue{gates: map[string]chan struct{}{}, logger: logx.Of(logger)}
}

// noteWait records a call that spent time in the queue rather than upstream. It
// answers the one question a stalled lane otherwise cannot: whether a slow request
// was slow at the provider or slow waiting for a slot the provider's per-session
// quota forces the gateway to serialize.
func (q *sessionQueue) noteWait(ctx context.Context, providerID, endpointID string, queued int, waited time.Duration, err error) {
	if waited < sessionWaitWarnBounds && err == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("provider_id", providerID),
		slog.String("endpoint_id", endpointID),
		slog.Int64("waited_ms", waited.Milliseconds()),
		slog.Int("queued_calls", queued),
	}
	event := "a call waited for its upstream session slot"
	if err != nil {
		event = "a call gave up waiting for its upstream session slot"
		attrs = append(attrs, slog.String("reason", err.Error()))
	}
	q.logger.LogAttrs(ctx, slog.LevelWarn, event, attrs...)
}

// acquire takes the session's slot, waiting for the holder to release it. A
// blocked send queues in FIFO order (Go channels are FIFO), so the requests
// waiting on one session are served in the order they arrived rather than in
// whatever order the scheduler wakes them. A canceled waiter leaves the queue
// without taking the slot, which is what keeps a client that hung up while
// queued from blocking the ones behind it.
//
// It reports how many calls were queued when this one arrived, counted before the
// wait rather than after: read afterwards it answers zero for every stall, which
// is the one fact the log line exists to carry.
func (q *sessionQueue) acquire(ctx context.Context, session string) (func(), int, error) {
	q.mu.Lock()
	gate, ok := q.gates[session]
	if !ok {
		gate = make(chan struct{}, 1)
		q.gates[session] = gate
	}
	queued := q.waiters + 1
	q.waiters = queued
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		q.waiters--
		q.mu.Unlock()
	}()

	select {
	case gate <- struct{}{}:
		// A canceled caller may win the race against the slot coming free; it
		// must not hold the slot for an attempt that is already dead.
		if err := ctx.Err(); err != nil {
			<-gate
			return nil, queued, err
		}
	case <-ctx.Done():
		return nil, queued, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-gate }) }, queued, nil
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
	release, queued, err := t.sessions.acquire(ctx, key)
	waited := time.Since(started)
	t.sessions.noteWait(ctx, call.Provider.ID, call.Credential.EndpointID(), queued, waited, err)
	if err != nil {
		return nil, err
	}
	if call.Budget != nil {
		call.Budget.pause(waited)
	}
	return release, nil
}

// sessionWaitTimeout reports a call that expired waiting for a session slot. The
// code stays UPSTREAM_TIMEOUT because clients branch on it and nothing about the
// contract says the upstream was at fault; the message does, because a request that
// never left the gateway must not be reported as a provider that did not answer.
func sessionWaitTimeout(cause error) error {
	return wrapDataPlaneError(CodeUpstreamTimeout, "the gateway's session queue did not free a slot in time", cause)
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
