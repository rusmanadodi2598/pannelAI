// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_refusal.go
// @for       The one log line an upstream's refusal of the request itself leaves behind, naming what it refused.
// @uses      context, log/slog, internal/domain, internal/logx, internal/requestctx.
// @reason    A request-shaped refusal is the only failure that leaves no trace anywhere: it parks no credential (selection_health.go RecordFailure returns before the write), stops the chain (engine_failure.go failoverWorthy), and the access line carries the code without the reason (router/middleware.go drops the message because the vendor's text can quote a credential back). An operator is then told UPSTREAM_REJECTED and given nothing to point at. This writes the identifiers the fix needs, and stops short of the text that cannot be logged.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-19
package dataplane

import (
	"context"
	"log/slog"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/logx"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/requestctx"
)

// logRequestRefusal records an upstream answer that refused this request as a
// request, which is the one class no other path writes anything about. It stays
// quiet on every other class: a rate limit or a transient outage already parks a
// credential, and the panel shows that, so a line per failover attempt would be
// noise over an account the pool is already walking.
//
// Only identifiers go on the line. The vendor's own message is what the client
// receives and what the access log deliberately withholds, so this one withholds
// it too.
func (e *Engine) logRequestRefusal(ctx context.Context, resolution Resolution, selection Selection, cause error) {
	failure, ok := AsUpstreamError(cause)
	if !ok || failureClass(cause) != domain.KeyFailureRequest {
		return
	}
	attrs := []slog.Attr{
		slog.String("request_id", requestctx.From(ctx)),
		slog.String("provider_id", resolution.Provider.ID),
		slog.String("model", resolution.ModelID),
		slog.String("upstream_model", resolution.UpstreamID),
		slog.String("endpoint_id", selection.Endpoint.ID()),
		slog.String("key_id", selection.Key.ID()),
		// The status the vendor stated, which for an endpoint that answers HTTP 200
		// and wraps its real code in the stream is the code inside the envelope.
		slog.Int("upstream_status", failure.Status),
	}
	// A combo member and a directly addressed model need different fixes, and the
	// signature names which vendor pattern read the status in the first place.
	if combo := resolution.Combo.Name(); combo != "" {
		attrs = append(attrs, slog.String("combo", combo))
	}
	if failure.SignatureID != "" {
		attrs = append(attrs, slog.String("signature_id", failure.SignatureID))
	}
	logx.Of(e.logger).LogAttrs(ctx, slog.LevelWarn, "upstream refused the request itself", attrs...)
}
