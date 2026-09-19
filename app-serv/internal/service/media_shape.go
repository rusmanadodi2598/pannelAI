// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_shape.go
// @for       How a media request addresses a provider: the model string, the
//
//	kind's declaration, and the format gate.
//
// @uses      internal/dataplane, internal/domain, internal/registry, strings.
// @reason    These rules are what "routable" means for a media call, and they
//
//	are separate from the call pipeline because they answer questions a
//	route may ask before it builds a payload. Keeping them apart also
//	keeps media_call.go inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Block returns one provider's declaration for a kind, with the format gate
// applied. A route whose payload depends on the declaration — search reads the
// parameter names it must send — needs it before it can call Prepare.
func (s *MediaCallService) Block(providerID string, kind domain.MediaKind) (registry.MediaConfig, error) {
	_, media, err := s.providerBlock(providerID, kind)
	return media, err
}

// providerBlock resolves the registry entry and the kind's declaration, and
// refuses a provider that does not offer the kind or declares a media format
// the gateway cannot speak.
func (s *MediaCallService) providerBlock(providerID string, kind domain.MediaKind) (registry.Provider, registry.MediaConfig, error) {
	entry, ok := s.index.Provider(providerID)
	if !ok {
		return registry.Provider{}, registry.MediaConfig{},
			dataplane.ErrNotFound("provider " + providerID + " is not in the registry")
	}
	media, ok := entry.Media.For(registryMediaKind(kind))
	if !ok {
		return registry.Provider{}, registry.MediaConfig{},
			dataplane.ProviderNotRoutable("provider " + entry.ID + " does not offer " + string(kind))
	}
	if !mediaFormatSupported(kind, media.Format) {
		return registry.Provider{}, registry.MediaConfig{}, dataplane.ProviderNotRoutable(
			"provider " + entry.ID + " declares the " + strings.TrimSpace(media.Format) + " " +
				string(kind) + " format, which the gateway does not translate yet")
	}
	return entry, media, nil
}

// kindNeedsModel reports whether a kind's upstream payload carries a model.
// Search does not: its providers read a query and a result count, so a
// declaration without a default model is complete rather than incomplete.
func kindNeedsModel(kind domain.MediaKind) bool {
	return kind != domain.MediaKindSearch
}

// splitMediaModel splits `provider/model` on the first slash, so a model id that
// itself carries one (openrouter's `openai/gpt-4o-mini-tts`) still resolves.
func splitMediaModel(model string) (string, string, error) {
	trimmed := strings.TrimSpace(model)
	providerID, upstreamModel, found := strings.Cut(trimmed, "/")
	if !found || strings.TrimSpace(providerID) == "" {
		return "", "", dataplane.ErrNotFound(
			"model " + trimmed + " is not routable: media routes expect provider/model")
	}
	return strings.TrimSpace(providerID), strings.TrimSpace(upstreamModel), nil
}

// mediaFormatSupported reports whether the gateway speaks a provider's declared
// media format for a kind.
//
// An empty declaration and `openai` are the OpenAI shape these routes build.
// Deepgram STT and NVIDIA NIM TTS are the first provider-specific adapters
// ported in G5, so their binary/JSON requests and provider-specific answers are
// handled separately. Other formats are refused by name instead of being sent
// a payload their API does not accept:
// the reference has a per-provider adapter for each of those formats, and
// reporting the gap is honest where a wrong-shaped call would look like an
// upstream outage. The search kind is exempt because its shape is built from
// the block's own declarations (method, parameter names) rather than from a
// fixed payload.
func mediaFormatSupported(kind domain.MediaKind, format string) bool {
	if kind == domain.MediaKindSearch {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "openai":
		return true
	case "deepgram":
		return kind == domain.MediaKindSTT
	case "nvidia-tts":
		return kind == domain.MediaKindTTS
	default:
		return false
	}
}

// mergeHeaders layers a request's own headers over the target's, so a caller
// can set what the kind's block does not (a multipart content type, say).
func mergeHeaders(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for name, value := range base {
		merged[name] = value
	}
	for name, value := range overrides {
		merged[name] = value
	}
	return merged
}
