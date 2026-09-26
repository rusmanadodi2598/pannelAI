// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/media.go
// @for       The media call surface: one outbound request to a non-chat service,
//
//	with the URL and headers its own kind declares.
//
// @uses      internal/provider, internal/registry, bytes, context, io, net/http,
//
//	net/url, strings, time.
//
// @reason    SPEC-API-001 §8.1 requires a media service's credential placement to
//
//	come from its per-kind block, because `auth_header: key` is a QUERY
//	PARAMETER rather than a header. Keeping the call and the target
//	construction in one file is what stops a caller from reaching for
//	the chat transport and authenticating incorrectly instead of
//	failing loudly; the placement branches live in media_credential.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// mediaBodyBytes bounds a media response body. An embeddings answer is a vector
// array, so the ceiling is generous but finite.
const mediaBodyBytes = 32 << 20

// MediaRequest is one outbound media call, already resolved: the URL, the headers,
// and the credential placement came from the kind's own configuration.
type MediaRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
	// ProviderID is the registry entry that answers, so the provider's own
	// proxy binding decides this call's route (docs/PORT/
	// 009-PORT-PROVIDER-PROXY.md D7). Empty follows the global setting.
	ProviderID string
	// TimeoutMS overrides the total deadline for this call, or zero for the
	// service default.
	TimeoutMS int
}

// MediaResponse is the bounded answer to one media call.
type MediaResponse struct {
	Status int
	Body   []byte
}

// MediaCaller performs one media request. It is an interface so the embeddings use
// case owns no transport, and a test drives the whole path without a network.
type MediaCaller interface {
	Do(ctx context.Context, request MediaRequest) (MediaResponse, error)
}

// MediaTransport is the HTTP implementation of MediaCaller.
type MediaTransport struct {
	dialer *ProxyDialer
}

// NewMediaTransport binds the caller to an HTTP client, defaulting to the shared
// pool configuration so a media call has the same §1.7 limits as a chat call.
// routes plans the proxy pool's attempts per destination; a nil value keeps the
// client's own routing.
func NewMediaTransport(client *http.Client, routes ProxyRoutePlanner) *MediaTransport {
	if client == nil {
		client = NewHTTPClient(HTTPClientDeps{})
	}
	return &MediaTransport{dialer: &ProxyDialer{client: client, routes: routes}}
}

// Do performs one media call under a context deadline (AGENTS.md §1.6).
func (t *MediaTransport) Do(ctx context.Context, request MediaRequest) (MediaResponse, error) {
	deadline := TotalTimeout
	if request.TimeoutMS > 0 {
		deadline = time.Duration(request.TimeoutMS) * time.Millisecond
	}
	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	built, err := http.NewRequestWithContext(callCtx, methodOrPost(request.Method), request.URL, bytes.NewReader(request.Body))
	if err != nil {
		return MediaResponse{}, wrapDataPlaneError(CodeUpstreamError, "the media request could not be built", err)
	}
	for key, value := range request.Headers {
		built.Header.Set(key, value)
	}

	response, err := t.dialer.Do(callCtx, built, request.ProviderID)
	if err != nil {
		if callCtx.Err() != nil {
			return MediaResponse{}, timeoutError(err)
		}
		return MediaResponse{}, wrapDataPlaneError(CodeUpstreamError, "the media upstream could not be reached", err)
	}
	defer func() {
		// reason: the body is drained below, so a close error here reports nothing
		// a caller could act on; the connection is released either way.
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, mediaBodyBytes))
	if err != nil {
		// reason: a partial error body names the failure as well as an empty one
		// does, so the read error must not mask the status the client acts on.
		body = nil
	}
	return MediaResponse{Status: response.StatusCode, Body: body}, nil
}

// methodOrPost reports the configured method, defaulting to POST.
func methodOrPost(declared string) string {
	trimmed := strings.TrimSpace(declared)
	if trimmed == "" {
		return http.MethodPost
	}
	return strings.ToUpper(trimmed)
}

// MediaTarget builds the URL and headers for one media call, placing the
// credential the way the kind's block declares it (media_credential.go).
//
// `auth_header: key` means a QUERY PARAMETER (SPEC-API-001 §8.1): measured in the
// reference, Gemini's embedding config uses a query-param key while its chat
// transport uses a header, so treating the declaration as a header name would
// authenticate incorrectly — and an upstream answers 401 for a reason the operator
// cannot see, which is worse than a loud failure.
func MediaTarget(media registry.MediaConfig, baseURL string, cred provider.Credential, query map[string]string) (string, map[string]string, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	for key, value := range media.Headers {
		headers[key] = value
	}
	target := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if target == "" {
		return "", nil, wrapDataPlaneError(CodeValidation, "the media service has no base_url configured", nil)
	}
	for name, value := range query {
		target = withQuery(target, name, value)
	}
	credentialed, err := applyMediaCredential(target, headers, media, credentialValue(cred))
	if err != nil {
		return "", nil, err
	}
	return credentialed, headers, nil
}

// MediaPath appends a path segment to a media target, keeping any query the
// credential placement already added.
//
// A provider whose endpoint names the model or the voice in its path (Gemini,
// ElevenLabs) shapes its own URL, and a naive concatenation would append after
// a `?key=` and build an address nothing answers.
func MediaPath(target, suffix string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return "", wrapDataPlaneError(CodeUpstreamError, "the media target could not be read", err)
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + suffix
	// String() re-encodes the path, and a stale RawPath would win over the
	// value just set; clearing it keeps the suffix in the path where it belongs.
	parsed.RawPath = ""
	return parsed.String(), nil
}

// credentialValue prefers the OAuth token when the account holds one, matching the
// chat transport's rule so one account presents the same credential family on both
// paths.
func credentialValue(cred provider.Credential) string {
	if cred.AccessToken != "" {
		return cred.AccessToken
	}
	return cred.APIKey
}

// withQuery adds a query parameter, keeping any the base URL already carries.
func withQuery(raw, name, value string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	query.Set(name, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
