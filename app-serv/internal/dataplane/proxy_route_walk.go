// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/proxy_route_walk.go
// @for       The pool walk's own judgements: which failure is worth another address, how long a refusing address sits out, and what the walk records about the address that answered.
// @uses      internal/domain, internal/provider, context, errors, fmt, net, net/http, net/url, strings, time.
// @reason    Measured against the live OpenCode free lane on 2026-10-15: bursting one egress address produced 429 while a second address carrying the same session and the same model served three requests in a row, and the first was still refusing eleven seconds later. The quota bucket is the address, so a refusal of that one status is the walk's business rather than the client's answer, and no other status is. Kept apart from proxy_route.go because the dial itself is one concern and the reading of what came back is another (AGENTS.md §1.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
)

// addressRefusal is what the walk needs from an answer that arrived: whether the
// next address is worth its round trip, and how long this one is skipped by the
// requests that follow.
type addressRefusal struct {
	failover bool
	parkFor  time.Duration
}

// classifyAddressRefusal reads the one status the pool owns. A 429 says this
// address asked too often, which a different address answers differently; a 403,
// a 404 or a 5xx describes the request or the vendor, and every candidate would
// refuse it the same way. The wait follows the vendor's own statement when it
// makes one, trimmed to the pool's cooldown so a header naming a day cannot take
// a working address out of rotation for a day.
func classifyAddressRefusal(response *http.Response) addressRefusal {
	if response == nil || response.StatusCode != http.StatusTooManyRequests {
		return addressRefusal{}
	}
	return addressRefusal{failover: true, parkFor: addressParkFor(response.Header)}
}

// addressParkFor is how long later requests skip an address that just refused. A
// vendor that names no wait gets the documented cooldown, which is the same
// reading the connect-stage failure already uses.
func addressParkFor(header http.Header) time.Duration {
	wait := provider.RetryAfter(header)
	if wait <= 0 || wait > domain.ProxyFailureCooldown {
		return domain.ProxyFailureCooldown
	}
	return wait
}

// noteServed records the address that carried the request. Spread is the pool's
// entire purpose and nothing else in the process says which candidate served a
// call, so an operator weighing a rotation change has no evidence without it. A
// walk that had to skip addresses is the event worth reading, not the routine one.
func (d *ProxyDialer) noteServed(ctx context.Context, providerID, candidateID, host string,
	status int, skipped []string) {
	attrs := []slog.Attr{
		slog.String("provider_id", providerID),
		slog.String("destination", host),
		slog.String("candidate", candidateID),
		slog.Int("status", status),
		slog.Int("skipped_candidates", len(skipped)),
	}
	logger := logx.Of(d.logger)
	if len(skipped) == 0 {
		logger.LogAttrs(ctx, slog.LevelDebug, "the proxy walk served the request", attrs...)
		return
	}
	attrs = append(attrs, slog.String("skipped_ids", strings.Join(skipped, ",")))
	logger.LogAttrs(ctx, slog.LevelWarn, "the proxy walk refused an address before serving", attrs...)
}

// proxyConnectRejected marks a candidate whose CONNECT the proxy answered with a
// non-200 status (docs/PORT/008-PORT-PROXY-ENGINE.md D7): the tunnel never
// opened, so the request was not delivered and the walk may spend the next
// candidate. Go reports that answer as a plain error, which the classifier
// below would otherwise read as the upstream's failure.
type proxyConnectRejected struct {
	status int
}

func (e *proxyConnectRejected) Error() string {
	return fmt.Sprintf("the proxy refused the CONNECT with status %d", e.status)
}

// rejectConnectResponse is the transport hook every candidate's clone carries:
// a proxy that does not answer 200 to the CONNECT is recorded as the typed
// rejection the walk fails over on.
func rejectConnectResponse(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
	if response.StatusCode != http.StatusOK {
		return &proxyConnectRejected{status: response.StatusCode}
	}
	return nil
}

// isProxyConnectFailure classifies the failures the walk may fail over on: a
// proxy that refused the CONNECT (the typed rejection the hook records), a
// dial error through the proxy (the OpError the transport wraps in url.Error)
// or a timeout waiting for it. A TLS failure the destination answered for and
// every mid-response error are the upstream's, not the pool's.
func isProxyConnectFailure(err error) bool {
	if err == nil {
		return false
	}
	var rejected *proxyConnectRejected
	if errors.As(err, &rejected) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
