// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/systemone_target.go
// @for       The URL and headers one decision call presents: the entry's own
//
//	block, the account's credential, and the session the Zen lanes require.
//
// @uses      internal/dataplane, internal/registry, net/url, strings.
// @reason    The reference builds these from `systemoneConfig` plus the account
// //
//
//	(systemoneCore.js:39-46): the block's headers, the credential as a
//	bearer, and a fresh `x-opencode-session` per call. Keeping them apart
//	from the use case is what holds systemone.go inside the AGENTS.md §1.1
//	budget, and it makes the one place a header is decided reviewable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package service

import (
	"net/url"
	"sort"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// systemOneTarget builds the absolute URL and the headers one decision call
// presents.
//
// The entry's block supplies the URL and its identity headers, and the account
// supplies the credential: the reference sends `Authorization: Bearer <token>`
// when the account holds one and nothing when it does not, which is what makes
// the free lane work without a stored key (systemoneCore.js:39-44). The
// credential is never placed in the URL, because a decision endpoint reads it
// from the header and a URL is the surface that ends up in a log.
func systemOneTarget(config *registry.SystemOneConfig, selection dataplane.Selection) (string, map[string]string, error) {
	target := strings.TrimSpace(config.BaseURL)
	if target == "" {
		return "", nil, dataplane.ValidationError("the decision endpoint declares no base_url")
	}
	if _, err := url.Parse(target); err != nil {
		return "", nil, dataplane.ValidationError("the decision endpoint URL could not be read")
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		return "", nil, dataplane.ValidationError("the decision endpoint URL is not absolute")
	}

	headers := map[string]string{"Content-Type": "application/json"}
	// The block's own headers are copied in a stable order so a test comparing
	// the outbound request is not map-iteration dependent.
	keys := make([]string, 0, len(config.Headers))
	for key := range config.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		headers[key] = config.Headers[key]
	}
	if token := selection.Credential.APIKey; token != "" {
		headers["Authorization"] = "Bearer " + token
	} else if token := selection.Credential.AccessToken; token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	// The Zen lanes expect the official client session on every request. The
	// reference mints a fresh one per call here (systemoneCore.js:45-46,
	// `generateSessionId()`), so this reuses the connector's own canonical
	// generator scoped to the decision route: one shape, two callers.
	headers["x-opencode-session"] = provider.OpenCodeSession(selection.Endpoint.ID() + ":systemone")
	return target, headers, nil
}
