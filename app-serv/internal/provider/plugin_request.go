// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_request.go
// @for       The request a connector is asked to perform, the response it returns, and the accounting that travels with it.
// @uses      internal/registry, net/http.
// @reason    A connector decides how to authenticate and which URL to call, so it has to receive the resolved model and the already-translated body rather than re-deriving either. The body is streamed rather than buffered because a chat completion can be long and the core forwards it without needing the whole payload.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-09-21
package provider

import (
	"context"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Request is one call the core asks a connector to perform. It carries the
// already-translated body and the resolved upstream target, so a connector only
// decides how to authenticate and which URL to use.
type Request struct {
	// Provider is the registry entry the call belongs to.
	Provider registry.Provider

	// Model is the resolved model, exposed so a connector can apply a
	// per-model rule (an upstream id override, a region from the model id).
	Model registry.Model

	// Wire is the upstream format Body was translated into. A multi-endpoint
	// provider picks its endpoint by this value, so a connector reads the wire
	// the request actually is rather than guessing it from the model: a model
	// may declare one target format while the request was translated into
	// another.
	Wire string

	// Body is the upstream-shaped request payload. Translation happened before
	// this point, so a connector must not reinterpret it.
	Body []byte

	// Stream reports whether the caller asked for a streamed response. A
	// provider that only streams is told so here rather than guessing.
	Stream bool

	// Headers are the request headers already assembled from the registry entry.
	// A connector may add to them, never replace them.
	Headers http.Header

	// Credential is the account this call presents. It is here because a
	// provider's request shape is not always a function of the request alone:
	// Qoder's chat body carries a model configuration the vendor publishes only
	// to the authenticated account, so shaping that body needs the credential
	// that will sign the read. A connector must not put any part of it into
	// `Body`, the body leaves the process as the client's prompt would.
	Credential Credential

	// Context is the caller's, carried on the request because the Transformer
	// seam takes the request and no parameter list: a connector that reads
	// anything during shaping (Qoder's model catalogue) would otherwise have to
	// detach that read from the client, and an unanswered read would outlive the
	// request it was made for. The data plane sets it; a connector treats a nil
	// value as a caller that is not watching.
	Context context.Context
}

// Response is what a connector returns. The body is streamed rather than
// buffered, because a chat completion can be long and the core forwards it
// without needing the whole payload.
type Response struct {
	// Status is the upstream HTTP status.
	Status int

	// Header carries the upstream response headers the caller may rely on
	// (retry hints, content type).
	Header http.Header

	// Body must be closed by the caller.
	Body ReadCloser

	// Usage is the accounting a connector can read without parsing the body:
	// some providers report quota on a header, and some report tokens in a
	// trailer. A zero value means "unknown", not "zero tokens".
	Usage Usage
}

// Usage is the token accounting one upstream call reported.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int

	// Reported distinguishes "the upstream said zero" from "the upstream said
	// nothing". Accounting must not record a fabricated zero as a measurement.
	Reported bool
}

// ReadCloser is the minimal read surface a response body needs. It is declared
// here rather than using io.ReadCloser directly so the interface stays explicit
// about what a connector must return.
type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}
