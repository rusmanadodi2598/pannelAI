// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/headroom_endpoint.go
// @for       Building and redacting the external compression endpoint.
// @uses      errors, fmt, net/url, strings.
// @reason    SPEC-API-001 §7.9 accepts a base URL rather than a fixed host, so
//
// the endpoint must preserve an operator's path and query while adding
// the one documented compression path. Redaction is kept beside the
// builder because URL credentials and query tokens must not enter errors.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// headroomCompressPath is the endpoint the proxy documents for compression.
const headroomCompressPath = "/v1/compress"

// headroomEndpoint appends the compression path to the configured base URL,
// keeping any base path and query the operator set and dropping the fragment.
func headroomEndpoint(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("headroom: the configured url is not a URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("headroom: the configured url is not absolute")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + headroomCompressPath
	parsed.RawPath = ""
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String(), nil
}

// headroomEndpointLabel is an endpoint as a log line may show it: no
// credentials, no query, no fragment. The URL is operator input, so it can
// carry a token in its query, and an error string travels into logs.
func headroomEndpointLabel(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String()
}
