// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/dataplane_record.go
// @for       The accounting pair a served data-plane call writes: one usage row
//
//	and one request log, under the request's own identifier.
//
// @uses      internal/dataplane, internal/domain, context, strconv, time.
// @reason    SPEC-API-001 §7.15's pipeline ends with "usage + quota + log
//
//	recording", and the media plane has no token counts to record — the
//	owner's D3 decision (2026-09-19) is that a media call still produces
//	both rows, with tokens 0 and the kind's own per-query price where one
//	is declared. Writing that rule once here is what keeps the media and
//	embeddings services from disagreeing about what a recorded call looks
//	like, the way they would if each built its own row.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"strconv"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// RequestLogRecorder stores one request log row, applying the capture setting
// before the write. LogService implements it, so the capture decision stays in
// one place rather than being re-decided by each data-plane caller.
type RequestLogRecorder interface {
	Record(ctx context.Context, in domain.RequestLogInput) (domain.RequestLog, error)
}

// dataPlaneRecorder writes the accounting pair for one data-plane call. Every
// collaborator is optional: a deployment that wires none keeps serving, and a
// missing one is skipped rather than failing the call.
type dataPlaneRecorder struct {
	usage     UsageRecorder
	logs      RequestLogRecorder
	requestID RequestIDReader
	clock     func() time.Time
}

// newDataPlaneRecorder binds the recorder to its collaborators, defaulting the
// clock so a recorder built with none is still usable.
func newDataPlaneRecorder(usage UsageRecorder, logs RequestLogRecorder, requestID RequestIDReader) dataPlaneRecorder {
	return dataPlaneRecorder{usage: usage, logs: logs, requestID: requestID, clock: time.Now}
}

// record writes one usage row and one request log for a call that reached an
// upstream, whether it succeeded or failed.
//
// The bodies are left empty on purpose: the media plane's payloads are audio
// bytes or multipart uploads rather than text, and the reference stores request
// detail for chat only, so the capture setting has nothing to store here. The
// error text is the fixed English message without its wrapped cause, because a
// media URL can carry the credential as a query parameter (§8.1) and a stored
// log must not hold a secret.
//
// A recording failure is never returned: the client already has its answer, and
// failing a served call over bookkeeping would turn it into an error the client
// cannot act on.
func (r dataPlaneRecorder) record(ctx context.Context, outcome dataplane.Outcome, keyID, costUSD string, latencyMS int64, failure error) {
	if r.usage == nil && r.logs == nil {
		return
	}
	requestID := requestIDOrNew(ctx, r.requestID, r.clock)
	status := domain.UsageStatusSuccess
	code, message := "", ""
	if failure != nil {
		status = domain.UsageStatusError
		failure := dataplane.AsError(failure)
		code, message = failure.Code, failure.Message
	}
	if r.usage != nil {
		// reason: the accounting write must not fail a request the client has
		// already received an answer to; a lost row is reported by the panel's
		// totals, which is where an operator can act on it.
		_, _ = r.usage.Record(ctx, domain.UsageRecordInput{
			RequestID:    requestID,
			EndpointID:   outcome.EndpointID,
			ProviderID:   outcome.ProviderID,
			Model:        outcome.Model,
			GatewayKeyID: keyID,
			CostUSD:      costUSD,
			LatencyMS:    latencyMS,
			Status:       status,
			ErrorCode:    code,
		})
	}
	if r.logs != nil {
		// reason: same as above — the log is the second half of the accounting
		// pair, not a condition of the answer.
		_, _ = r.logs.Record(ctx, domain.RequestLogInput{
			RequestID:    requestID,
			GatewayKeyID: keyID,
			EndpointID:   outcome.EndpointID,
			ProviderID:   outcome.ProviderID,
			Model:        outcome.Model,
			Status:       requestLogStatus(status),
			LatencyMS:    latencyMS,
			Error:        logError(code, message),
		})
	}
}

// requestLogStatus mirrors the usage status onto the log's own closed set, so
// one call cannot be "success" on one row and "error" on the other.
func requestLogStatus(status domain.UsageStatus) domain.RequestLogStatus {
	if status.Success() {
		return domain.RequestLogSuccess
	}
	return domain.RequestLogError
}

// refuse writes the one request log row a call leaves when it was refused
// before any upstream attempt (register G20), so a media or embeddings refusal
// appears in the §7.13 Logs screen the way a chat refusal already does.
//
// No usage row is written: nothing was spent, and the usage aggregate requires a
// provider and a model — the same deliberate skip the chat plane makes for a
// call refused before the pipeline ran. The stored error text is the code
// alone: the message is ours and the row must not hold text an upstream can
// influence (the G6/G18 rule).
func (r dataPlaneRecorder) refuse(ctx context.Context, outcome dataplane.Outcome, keyID string, failure error) {
	if r.logs == nil || failure == nil {
		return
	}
	// reason: the refusal's log row is bookkeeping, not a condition of the
	// client's error — a failed write retries on the next call.
	_, _ = r.logs.Record(ctx, domain.RequestLogInput{
		RequestID:    requestIDOrNew(ctx, r.requestID, r.clock),
		GatewayKeyID: keyID,
		EndpointID:   outcome.EndpointID,
		ProviderID:   outcome.ProviderID,
		Model:        outcome.Model,
		Status:       domain.RequestLogError,
		Error:        dataplane.AsError(failure).Code,
	})
}

// logError renders the code and its message for a stored log row, or an empty
// string for a call that succeeded.
func logError(code, message string) string {
	if code == "" {
		return ""
	}
	return code + ": " + message
}

// requestIDOrNew reads the router's request id, falling back to a fresh ULID so
// a row always carries one (the column is not nullable in practice, and a blank
// id would make the row unfindable).
func requestIDOrNew(ctx context.Context, reader RequestIDReader, clock func() time.Time) string {
	if reader != nil {
		if id := reader(ctx); id != "" {
			return id
		}
	}
	return domain.NewULID(clock())
}

// mediaCost renders a media kind's per-query price as the decimal string a
// usage row stores. Only search declares one today; a kind without a price
// records 0, which is what "this call has no per-query cost" means.
func mediaCost(perQuery float64) string {
	if perQuery == 0 {
		return "0"
	}
	return strconv.FormatFloat(perQuery, 'f', -1, 64)
}
