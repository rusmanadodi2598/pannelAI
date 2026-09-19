// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/headroom.go
// @for       The external compression client: one OpenAI-shaped message array
//
//	sent to a Headroom proxy's /v1/compress, the compressed array read
//	back, and every failure returned to a caller that keeps what it had.
//
// @uses      context, encoding/json, errors, fmt, io, net/http, net/url,
//
//	strings, time.
//
// @reason    SPEC-API-001 §7.9 makes headroom the one saver that leaves the
//
//	process, and requires it to fail open. Keeping the call here, apart
//	from the pipeline that decides when to make it, is what keeps those
//	two concerns from drifting: the client has no opinion about which
//	wire the body arrived in, and the caller has no opinion about how
//	the proxy is reached.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HeadroomClient calls a Headroom proxy's compression endpoint. The HTTP client
// is injected because every outbound call the gateway makes shares one guarded
// client (OWASP A01); the timeout is the saver's own, because it bounds one
// optional call rather than the connection pool.
type HeadroomClient struct {
	client  *http.Client
	timeout time.Duration
}

// NewHeadroomClient returns a client that abandons a compression call after
// HeadroomDefaultTimeout and leaves the request untouched.
func NewHeadroomClient(client *http.Client) *HeadroomClient {
	return &HeadroomClient{client: client, timeout: HeadroomDefaultTimeout}
}

// HeadroomRequest is one compression call. Messages must already be an OpenAI
// message array: the proxy speaks that shape only, so a caller holding another
// wire translates into it before the call and back out of it after.
type HeadroomRequest struct {
	// URL is the proxy base, e.g. http://localhost:8787. The compression path
	// is appended to it.
	URL string
	// Model is the model the request being compressed is addressed to. The
	// proxy may use it to pick a tokenizer; it is sent as configured, including
	// when it is empty.
	Model string
	// Messages is the message array to compress.
	Messages json.RawMessage
	// CompressUserMessages asks the proxy to rewrite user messages too. The
	// proxy leaves them alone by default, and the saver keeps that default.
	CompressUserMessages bool
}

// HeadroomResult is what a successful call returns. Messages is the proxy's
// array, carried as it arrived; the token counts are the proxy's own
// accounting, and are zero when it did not report them.
type HeadroomResult struct {
	Messages     json.RawMessage
	TokensBefore int
	TokensAfter  int
	TokensSaved  int
}

// headroomPayload is the body /v1/compress documents: the message array, the
// model, and the one optional config member.
type headroomPayload struct {
	Messages json.RawMessage  `json:"messages"`
	Model    string           `json:"model"`
	Config   *headroomOptions `json:"config,omitempty"`
}

// headroomOptions is the optional config member.
type headroomOptions struct {
	CompressUserMessages bool `json:"compress_user_messages"`
}

// headroomAnswer is the part of the proxy's response the client reads.
type headroomAnswer struct {
	Messages     json.RawMessage `json:"messages"`
	TokensBefore int             `json:"tokens_before"`
	TokensAfter  int             `json:"tokens_after"`
	TokensSaved  int             `json:"tokens_saved"`
}

// Compress posts the message array and returns the proxy's compressed array.
//
// Every failure is an error, and no failure is fatal to the request: the caller
// keeps the body it already had, which is what SPEC-API-001 §7.9 means by fails
// open. A refusal and an unreachable proxy are both errors because the caller
// does the same thing either way.
func (c *HeadroomClient) Compress(ctx context.Context, req HeadroomRequest) (HeadroomResult, error) {
	if len(req.Messages) == 0 {
		return HeadroomResult{}, errors.New("headroom: no messages to compress")
	}
	endpoint, err := headroomEndpoint(req.URL)
	if err != nil {
		return HeadroomResult{}, err
	}
	payload := headroomPayload{Messages: req.Messages, Model: req.Model}
	if req.CompressUserMessages {
		payload.Config = &headroomOptions{CompressUserMessages: true}
	}
	encoded, err := marshalNoEscape(payload)
	if err != nil {
		return HeadroomResult{}, fmt.Errorf("headroom: encoding the request: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, c.callTimeout())
	defer cancel()
	call, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return HeadroomResult{}, fmt.Errorf("headroom: building the request: %w", err)
	}
	call.Header.Set("Content-Type", "application/json")

	answer, err := c.call(call)
	if err != nil {
		return HeadroomResult{}, err
	}
	return answer, nil
}

// call performs the exchange and reads the answer. It is separate from Compress
// so the request that is sent and the answer that is accepted can each be read
// in one place.
func (c *HeadroomClient) call(call *http.Request) (HeadroomResult, error) {
	label := headroomEndpointLabel(call.URL.String())
	response, err := c.client.Do(call)
	if err != nil {
		return HeadroomResult{}, fmt.Errorf("headroom: calling %s: %w", label, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return HeadroomResult{}, fmt.Errorf("headroom: %s answered HTTP %d", label, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, headroomMaxResponseBytes+1))
	if err != nil {
		return HeadroomResult{}, fmt.Errorf("headroom: reading the answer from %s: %w", label, err)
	}
	if len(body) > headroomMaxResponseBytes {
		return HeadroomResult{}, fmt.Errorf("headroom: %s answered with more than %d bytes", label, headroomMaxResponseBytes)
	}
	var answer headroomAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return HeadroomResult{}, fmt.Errorf("headroom: the answer from %s is not JSON: %w", label, err)
	}
	if _, ok := decodeArray(answer.Messages); !ok {
		return HeadroomResult{}, fmt.Errorf("headroom: the answer from %s carries no messages array", label)
	}
	return HeadroomResult(answer), nil
}

// callTimeout is the timeout in force for one call. A client built without one
// falls back to the documented default rather than to an immediate deadline,
// which is the reference's normalizeTimeout.
func (c *HeadroomClient) callTimeout() time.Duration {
	if c.timeout > 0 {
		return c.timeout
	}
	return HeadroomDefaultTimeout
}
