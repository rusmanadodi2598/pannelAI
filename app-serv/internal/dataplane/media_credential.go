// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/media_credential.go
// @for       Where one media call's credential goes: the placement each kind's
//
//	block declares, and the schemes that are not a plain bearer.
//
// @uses      internal/provider, internal/registry, strings.
// @reason    SPEC-API-001 §8.1 makes the per-kind block the source of truth for
//
//	credential placement, and the branches grew past a plain
//	bearer/query pair: Basic (Inworld), PlayHT's user id plus key,
//	and a declared header name (ElevenLabs). Keeping them in one
//	file means a new scheme is one case here rather than a second
//	reader of the same declaration, and media.go stays inside the
//	AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// applyMediaCredential places the credential the way the kind's block declares
// it, returning the target a query-parameter scheme rewrote.
//
// An empty secret means the provider needs none, so no header is written at
// all: the chat path's ApplyAuth documents the same rule, and an empty bearer
// is a malformed credential rather than an anonymous call (G16).
func applyMediaCredential(target string, headers map[string]string, media registry.MediaConfig, secret string) (string, error) {
	declared := strings.ToLower(strings.TrimSpace(media.AuthHeader))
	switch {
	case media.AuthType == "" || media.AuthType == registry.AuthNone || declared == "none":
		return target, nil
	case declared == "key" || declared == "query":
		if secret == "" {
			return "", wrapDataPlaneError(CodeUpstreamError, "the media service has no credential", nil)
		}
		return withQuery(target, "key", secret), nil
	case declared == "bearer" || declared == "token":
		if secret != "" {
			scheme := "Bearer"
			if declared == "token" {
				scheme = "Token"
			}
			headers["Authorization"] = scheme + " " + secret
		}
		return target, nil
	case declared == "basic":
		if secret != "" {
			headers["Authorization"] = "Basic " + secret
		}
		return target, nil
	case declared == "playht":
		applyPlayHT(headers, secret)
		return target, nil
	case declared == "":
		// A kind that declares an auth type but no header keeps the registry's
		// documented default, which is Authorization + bearer.
		if secret != "" {
			headers[provider.DefaultAuthHeader] = "Bearer " + secret
		}
		return target, nil
	default:
		if secret != "" {
			headers[canonicalHeader(media.AuthHeader)] = secret
		}
		return target, nil
	}
}

// applyPlayHT splits PlayHT's `userId:apiKey` credential into the two headers
// its API reads. A value without the separator is sent as a bearer alone: the
// user id is half of a pair, and an empty one is a malformed credential (G16).
func applyPlayHT(headers map[string]string, secret string) {
	if secret == "" {
		return
	}
	userID, key, paired := strings.Cut(secret, ":")
	if !paired || strings.TrimSpace(key) == "" {
		headers["Authorization"] = "Bearer " + secret
		return
	}
	if userID = strings.TrimSpace(userID); userID != "" {
		headers["X-USER-ID"] = userID
	}
	headers["Authorization"] = "Bearer " + key
}

// canonicalHeader renders a declared header name in its canonical spelling, so the
// outbound request carries `Xi-Api-Key` rather than a lowercase spelling an upstream
// may compare exactly.
func canonicalHeader(name string) string {
	parts := strings.Split(name, "-")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, strings.ToUpper(part[:1])+strings.ToLower(part[1:]))
	}
	return strings.Join(out, "-")
}
