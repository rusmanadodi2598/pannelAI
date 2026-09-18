// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/upstream.go
// @for       The upstream call shape, its failure type, and the reader helpers
//
//	that keep a response bounded.
//
// @uses      internal/provider, encoding/json, io, net/http.
// @reason    The transport, the handler, and the tests all need the same three
//
//	things — what a call is, how an upstream failure is reported, and
//	how a body is read without trusting its length. Declaring them in
//	one file keeps the transport file about policy and this file about
//	shape.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Call is one upstream request the data plane wants performed. Its fields are
// already resolved: translation happened before this point, so the transport
// forwards the body it is given.
type Call struct {
	// Provider is the registry entry that will answer.
	Provider registry.Provider
	// Model is the resolved model, exposed so a connector can apply a
	// per-model rule.
	Model registry.Model
	// Credential is the account material for this one call.
	Credential provider.Credential
	// Body is the upstream-shaped payload.
	Body []byte
	// Stream reports whether the caller asked for a streamed response.
	Stream bool
	// Idempotent reports whether repeating the request is safe. A chat
	// completion is not: the upstream may have started the work.
	Idempotent bool
}

// Upstream is a successful answer: the status, the headers, the accounting a
// connector could decode, and the body the caller closes.
type Upstream struct {
	Status int
	Header http.Header
	Usage  provider.Usage
	Body   io.ReadCloser
}

// Close releases the body. A nil body is a no-op, so a caller can defer this
// without a nil check.
func (u *Upstream) Close() error {
	if u == nil || u.Body == nil {
		return nil
	}
	return u.Body.Close()
}

// UpstreamError is a non-2xx upstream answer, carrying what the caller needs to
// classify it: the status for the retry decision and the body for the plugin's
// quota check.
type UpstreamError struct {
	Status  int
	Header  http.Header
	Message string
	// Body is bounded and kept for classification and for the log; it never
	// reaches the client verbatim (AGENTS.md §1.3).
	Body []byte
}

// Error renders the status and the upstream message.
func (e *UpstreamError) Error() string {
	return "upstream returned " + http.StatusText(e.Status) + ": " + e.Message
}

// AsUpstreamError extracts an upstream failure, or reports false.
func AsUpstreamError(err error) (*UpstreamError, bool) {
	var failure *UpstreamError
	if errors.As(err, &failure) {
		return failure, true
	}
	return nil, false
}

// RequestFor builds the plugin's view of a call. It is exported so a test can
// assert the seam receives the resolved model and the stream flag.
func RequestFor(call Call) provider.Request {
	return provider.Request{
		Provider: call.Provider,
		Model:    call.Model,
		Body:     call.Body,
		Stream:   call.Stream,
	}
}

// readBounded reads at most limit bytes and reports an error when the body is
// larger, so a provider cannot exhaust memory through one response.
func readBounded(body io.Reader, limit int64) ([]byte, error) {
	buffered, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buffered)) > limit {
		return nil, errors.New("the upstream response exceeded the size limit")
	}
	return buffered, nil
}

// readUpstreamError drains a bounded part of a failed response, so the plugin's
// quota check and the log have something to read and the connection is reusable.
//
// The accounting a failing response reported is deliberately not attached: a
// rejected call has no billed tokens, and reporting a number the upstream did not
// measure would fabricate a measurement.
func readUpstreamError(response *http.Response) *UpstreamError {
	defer func() {
		// reason: the body is drained below, so a close error carries nothing a
		// caller could act on; the connection is released either way.
		_ = response.Body.Close()
	}()
	body, err := readBounded(response.Body, errorBodyBytes)
	if err != nil {
		// reason: a partial error body names the failure as well as an empty one
		// does, so the read error must not mask the status.
		body = nil
	}
	return &UpstreamError{
		Status:  response.StatusCode,
		Header:  response.Header,
		Message: upstreamMessage(response.StatusCode, body),
		Body:    body,
	}
}

// upstreamMessage extracts the message an upstream error body carries, falling
// back to the status text so the client always gets something actionable rather
// than an empty string.
func upstreamMessage(status int, body []byte) string {
	var shaped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if len(body) > 0 && json.Unmarshal(body, &shaped) == nil {
		if shaped.Error.Message != "" {
			return shaped.Error.Message
		}
		if shaped.Message != "" {
			return shaped.Message
		}
	}
	if text := http.StatusText(status); text != "" {
		return "the upstream rejected the request: " + text
	}
	return "the upstream rejected the request"
}
