// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_envelope.go
// @for       Streaming Qoder answers through the envelope unwrap: the peek that
//
//	decides the call, and the body the core then reads.
//
// @uses      bufio, errors, io, net/http.
// @reason    Qoder answers a chat with HTTP 200 and puts the real status inside
//
//	each frame (measured, draft 036 §5.1). Without unwrapping, a spent
//	account would be piped to the client as an answer and billed as one.
//	The first frame is therefore read before anything is piped, so a
//	refusal there becomes a failure the router can act on. Reading one
//	frame and deciding what its refusal means live in
//	qoder_envelope_frame.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package provider

import (
	"bufio"
	"errors"
	"io"
	"net/http"
)

// OpenStream implements provider.StreamEnvelope: it peeks the first frame and either
// reports the vendor's refusal or returns a body that yields plain OpenAI SSE.
//
// The frames after the first carry the same wrapper, so the unwrap is a line
// transform and the answer's own JSON survives byte-for-byte. That is what lets the
// core's stream state keep doing the job it already does, one finish_reason and one
// usage delivery, which is exactly Qoder's finish-then-usage pattern (draft 034 F2),
// instead of this connector growing a provider-specific coalescer.
func (c *Qoder) OpenStream(body io.ReadCloser) (io.ReadCloser, *StreamFailure) {
	if body == nil {
		return nil, &StreamFailure{Status: http.StatusBadGateway, Message: "the provider sent no stream"}
	}
	reader := bufio.NewReaderSize(body, 16<<10)
	first, _, err := readFirstSSEData(reader)
	if err != nil && !errors.Is(err, io.EOF) {
		_ = body.Close()
		return nil, &StreamFailure{Status: http.StatusBadGateway, Message: "the provider stream could not be read"}
	}
	if first == nil {
		// Nothing to unwrap: an empty body is the provider's answer, not a fault
		// of ours, and the stream state already renders no frames.
		return &qoderUnwrapped{body: body, reader: reader}, nil
	}
	envelope, parsed := parseQoderEnvelope(*first)
	if parsed && envelope.status() != http.StatusOK {
		_ = body.Close()
		return nil, qoderStreamFailure(envelope)
	}
	inner, ok := envelope.innerPayload()
	if !parsed || !ok {
		// A frame that is not an envelope at all passes through unchanged: a
		// provider that changes shape must not be met by a gateway that drops the
		// answer it did send.
		return &qoderUnwrapped{body: body, reader: reader, pending: appendSSEFrame(*first)}, nil
	}
	// The peeked frame is replayed unwrapped: the client sees no envelope, and
	// loses none of the answer that was inside it.
	return &qoderUnwrapped{body: body, reader: reader, pending: appendSSEFrame(inner)}, nil
}

// qoderUnwrapped is the response body the core reads: it yields the unwrapped SSE,
// starting with the frame the peek consumed, then unwrapping each frame after it.
type qoderUnwrapped struct {
	body    io.ReadCloser
	reader  *bufio.Reader
	pending []byte
	done    bool
}

func (r *qoderUnwrapped) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		if r.done {
			return 0, io.EOF
		}
		frame, err := r.nextFrame()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return 0, err
			}
			r.done = true
		}
		r.pending = frame
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *qoderUnwrapped) Close() error { return r.body.Close() }

// nextFrame reads one event and returns it unwrapped, or nil when it carries nothing
// worth forwarding.
func (r *qoderUnwrapped) nextFrame() ([]byte, error) {
	for {
		data, err := readSSEData(r.reader)
		if err != nil {
			return nil, err
		}
		if data == nil {
			continue
		}
		if string(data) == "[DONE]" {
			r.done = true
			return []byte("data: [DONE]\n\n"), nil
		}
		envelope, parsed := parseQoderEnvelope(data)
		if !parsed {
			return appendSSEFrame(data), nil
		}
		if envelope.status() != http.StatusOK {
			// A refusal after the stream started cannot change the status the
			// client already received, so the reference's answer stands: end the
			// answer with the provider's reason as content.
			r.done = true
			return qoderErrorFrame(envelope), nil
		}
		inner, ok := envelope.innerPayload()
		if !ok {
			continue
		}
		return appendSSEFrame(inner), nil
	}
}
