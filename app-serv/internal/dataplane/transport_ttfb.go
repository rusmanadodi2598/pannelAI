// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/transport_ttfb.go
// @for       The wait for a streamed answer's first byte: how long it may stay silent, and what a cap that fires reports.
// @uses      bytes, context, io, time.
// @reason    A streamed attempt has no total deadline (a long answer is not a failure), so a provider that accepts the call and then says nothing held the request until the client gave up. The idle guard cannot see this state because it only bounds the gaps after the first read. The wait is cut here, inside the attempt, rather than on the body: only the transport can turn it into a retry against the same target and a failover to the next credential, and only before any byte reached the caller is repeating the request safe.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// firstByteWithin reads the first byte of a streamed body under a cap. On
// success it returns a body that replays the byte already read, so the caller
// sees the stream exactly as the upstream wrote it. On the cap firing it
// cancels the attempt (which closes the connection the blocked read is on) and
// reports a timeout, which the retry loop may repeat because nothing was piped.
//
// The cancel is only ever called by the timer, and Stop is the decision point:
// a timer that could not be stopped has already canceled the attempt context,
// so the byte that raced it belongs to a dead request and is discarded rather
// than handed out as a stream that cannot be read.
func firstByteWithin(body io.ReadCloser, cap time.Duration, cancel context.CancelFunc) (io.ReadCloser, error) {
	done := make(chan struct{})
	timer := time.AfterFunc(cap, func() {
		cancel()
		close(done)
	})
	buf := make([]byte, 1)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if timer.Stop() {
				return &prefixedBody{reader: io.MultiReader(bytes.NewReader(buf[:n]), body), closer: body}, nil
			}
			<-done
			return nil, firstByteSilent(cap)
		}
		if err != nil {
			if timer.Stop() {
				return nil, wrapDataPlaneError(CodeUpstreamTimeout,
					"the upstream stream could not be read before its first byte", err)
			}
			<-done
			return nil, firstByteSilent(cap)
		}
		// A read of zero bytes with no error is allowed by the reader contract,
		// so the wait continues under the same cap rather than treating it as
		// progress.
	}
}

// firstByteSilent reports an attempt cut by the first-byte cap. It is a
// transport failure rather than an upstream refusal, so the retry decision
// classifies it like any other unreachable upstream and the walk may move on.
func firstByteSilent(cap time.Duration) error {
	return wrapDataPlaneError(CodeUpstreamTimeout,
		"the upstream stream stayed silent for "+cap.String()+" before its first byte", context.DeadlineExceeded)
}

// openEnvelopeWithin bounds the envelope peek that runs before the first-byte
// guard; a keepalive-only stream would otherwise hold the request and socket.
func openEnvelopeWithin(
	body io.ReadCloser,
	cap time.Duration,
	cancel context.CancelFunc,
	envelope provider.StreamEnvelope,
) (io.ReadCloser, *provider.StreamFailure, error) {
	if cap <= 0 {
		opened, failure := envelope.OpenStream(body)
		return opened, failure, nil
	}
	type answer struct {
		body    io.ReadCloser
		failure *provider.StreamFailure
	}
	result := make(chan answer, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("stream envelope peek panicked", "panic", recovered, "stack", string(debug.Stack()))
				result <- answer{failure: &provider.StreamFailure{
					Status:  http.StatusBadGateway,
					Message: "the provider stream could not be read",
				}}
			}
		}()
		opened, failure := envelope.OpenStream(body)
		result <- answer{body: opened, failure: failure}
	}()
	timer := time.NewTimer(cap)
	defer timer.Stop()
	select {
	case opened := <-result:
		return opened.body, opened.failure, nil
	case <-timer.C:
		cancel()
		_ = body.Close()
		return nil, nil, firstByteSilent(cap)
	}
}

// prefixedBody replays the byte the first-byte wait already read ahead of the
// body it came from, and closes that body.
type prefixedBody struct {
	reader io.Reader
	closer io.Closer
}

func (b *prefixedBody) Read(p []byte) (int, error) { return b.reader.Read(p) }

func (b *prefixedBody) Close() error { return b.closer.Close() }
