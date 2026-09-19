// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/media_provider.go
// @for       The media kind set and the per-provider override an operator saves
//
//	(SPEC-API-001 §7.10).
//
// @uses      net/url, strings, time.
// @reason    The kind set is closed because each member maps to one route and
//
//	one request shape, and the base URL is validated here because it
//	becomes an outbound destination: a shape check at the boundary is
//	what this layer can prove (OWASP A01). The dial-time guard
//	(`internal/netguard`) is wired for proxy candidates; upstream dials
//	— this one and the chat plane's alike — do not pass through it yet.
//	The registry's own kind names (`webSearch`) stay out of this package
//	— the mapping is the service's job, so the wire vocabulary and the
//	registry vocabulary cannot be confused for one another.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-19
package domain

import (
	"net/url"
	"strings"
	"time"
)

// MediaKind is the API's closed kind set for media providers (§7.10). The panel
// calls the search kind `web` and maps it to `search` before sending.
type MediaKind string

const (
	MediaKindTTS       MediaKind = "tts"
	MediaKindSTT       MediaKind = "stt"
	MediaKindEmbedding MediaKind = "embedding"
	MediaKindImage     MediaKind = "image"
	MediaKindVideo     MediaKind = "video"
	MediaKindSearch    MediaKind = "search"
)

// MediaKinds returns the set in a stable order, so a list route renders the
// same way twice.
func MediaKinds() []MediaKind {
	return []MediaKind{MediaKindTTS, MediaKindSTT, MediaKindEmbedding, MediaKindImage, MediaKindVideo, MediaKindSearch}
}

// Where an effective base URL or default model came from. The two values are
// resolved independently, so each reports its own source rather than sharing
// one "overridden" flag that would mislabel a model-only override.
const (
	MediaSourceRegistry = "registry"
	MediaSourceOverride = "override"
)

// ParseMediaKind validates a wire value against the closed set.
func ParseMediaKind(raw string) (MediaKind, error) {
	kind := MediaKind(strings.ToLower(strings.TrimSpace(raw)))
	for _, known := range MediaKinds() {
		if kind == known {
			return kind, nil
		}
	}
	return "", NewValidationError("kind must be one of tts, stt, embedding, image, video, search")
}

// MediaOverride is one stored per-kind override for a provider.
//
// An empty BaseURL means "use the registry's", which is why the value is a
// plain string rather than a pointer: one representation, read the same way by
// every caller.
type MediaOverride struct {
	providerID   string
	kind         MediaKind
	baseURL      string
	defaultModel string
	updatedAt    time.Time
}

// NewMediaOverride validates and builds an override.
func NewMediaOverride(providerID string, kind MediaKind, baseURL, defaultModel string, now time.Time) (MediaOverride, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return MediaOverride{}, NewValidationError("provider_id is required")
	}
	if _, err := ParseMediaKind(string(kind)); err != nil {
		return MediaOverride{}, err
	}
	normalized, err := normalizeMediaBaseURL(baseURL)
	if err != nil {
		return MediaOverride{}, err
	}
	defaultModel = strings.TrimSpace(defaultModel)
	if len(defaultModel) > 200 {
		return MediaOverride{}, NewValidationError("default_model must be at most 200 characters")
	}
	return MediaOverride{
		providerID: providerID, kind: kind, baseURL: normalized,
		defaultModel: defaultModel, updatedAt: now,
	}, nil
}

// RehydrateMediaOverride rebuilds a stored row. For the repository load path
// only; never use it to create an override.
func RehydrateMediaOverride(providerID string, kind MediaKind, baseURL, defaultModel string, updatedAt time.Time) MediaOverride {
	return MediaOverride{
		providerID: providerID, kind: kind, baseURL: baseURL,
		defaultModel: defaultModel, updatedAt: updatedAt,
	}
}

// Accessors expose state without allowing mutation.
func (o MediaOverride) ProviderID() string   { return o.providerID }
func (o MediaOverride) Kind() MediaKind      { return o.kind }
func (o MediaOverride) BaseURL() string      { return o.baseURL }
func (o MediaOverride) DefaultModel() string { return o.defaultModel }
func (o MediaOverride) UpdatedAt() time.Time { return o.updatedAt }

// normalizeMediaBaseURL validates an override and trims the trailing slashes
// the media transport would trim anyway, so the stored value is the value that
// is dialed.
//
// The shape rule refuses anything that is not an absolute http(s) URL with a
// host: a scheme the transport cannot dial (file, gopher, ftp) is refused here
// rather than at the first call, and whitespace is refused because a URL with
// an embedded newline is a header-injection shape, not a typo.
func normalizeMediaBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 2048 {
		return "", NewValidationError("base_url must be at most 2048 characters")
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return "", NewValidationError("base_url must not contain whitespace")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", NewValidationError("base_url must be an absolute http(s) URL")
	}
	switch {
	case parsed.Scheme == "":
		return "", NewValidationError("base_url must be an absolute http(s) URL")
	case parsed.Scheme != "http" && parsed.Scheme != "https":
		return "", NewValidationError("base_url must use the http or https scheme")
	case parsed.Host == "":
		return "", NewValidationError("base_url must be an absolute http(s) URL")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
