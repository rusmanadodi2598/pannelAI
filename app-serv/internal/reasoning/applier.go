// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/applier.go
// @for       The request-path seam: resolving one call's reasoning config and
//
//	writing it into the already translated upstream body.
//
// @uses      internal/domain, internal/registry, context, encoding/json, errors.
// @reason    SPEC-API-001 §7.14 stores one thinking mode per provider and §7.15
//
//	carries the reference's per-model suffix. Both are decisions about an
//	outbound body, and the engine hands this seam the body it has already
//	translated, so a config is written once, in the shape the upstream
//	reads, and never translated away afterwards.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-26
package reasoning

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// SettingsReader is the narrow settings port this seam reads. The service
// implementation owns persistence and defaults; the applier only needs the
// typed document.
type SettingsReader interface {
	Settings(context.Context) (domain.Settings, error)
}

// Call describes one outbound call's reasoning inputs: the wire the body is
// written in, the model the request resolved to, the client's own body (whose
// reasoning intent outranks the stored mode), and the model string's suffix
// (which outranks both).
type Call struct {
	Wire       string
	ProviderID string
	ModelID    string
	ClientRaw  []byte
	Override   *Suffix
}

// Applier writes the reasoning control into an outbound body.
type Applier struct {
	settings SettingsReader
}

// NewApplier validates the settings port and returns an applier.
func NewApplier(settings SettingsReader) (*Applier, error) {
	if settings == nil {
		return nil, errors.New("reasoning: settings reader is required")
	}
	return &Applier{settings: settings}, nil
}

// Apply resolves the call's reasoning config and writes it into body. It
// answers the body unchanged when the model does not reason, when no config is
// configured or requested, or when the body cannot be read — an optional
// control must never fail an otherwise valid upstream call.
func (a *Applier) Apply(ctx context.Context, body []byte, call Call) []byte {
	if a == nil || len(body) == 0 {
		return body
	}
	caps := registry.Capabilities(call.ProviderID, call.ModelID)
	// A model the resolver says does not reason gets no field from this port.
	// The client's own fields are left as they are: the reference strips them
	// for such a model, and this port deliberately does not, because its
	// capability answer is the curated tables' and a custom node's models are
	// not in them.
	if !caps.Reasoning {
		return body
	}
	cfg := a.resolve(ctx, call)
	if cfg == nil {
		return body
	}
	decoded, ok := decodeObject(body)
	if !ok {
		return body
	}
	format := resolveFormat(call.Wire, caps)
	if format == "" {
		return body
	}
	display := thinkingDisplay(decoded)
	stripAll(decoded)
	applyFormat(decoded, format, *cfg, modelCapsFrom(caps, call), display)
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return body
	}
	return encoded
}

// resolve answers the config one call carries, in the reference's precedence:
// the per-model suffix outranks the client's own intent, which outranks the
// stored provider mode. The settings read happens last, so a call that already
// carries an intent pays for nothing.
func (a *Applier) resolve(ctx context.Context, call Call) *Config {
	if call.Override != nil {
		if cfg := configFromSuffix(*call.Override); cfg != nil {
			return cfg
		}
	}
	if cfg := Extract(call.ClientRaw); cfg != nil {
		return cfg
	}
	settings, err := a.settings.Settings(ctx)
	if err != nil {
		return nil
	}
	mode, found := settings.Reasoning.ThinkingFor(call.ProviderID)
	if !found {
		return nil
	}
	return configFromMode(mode)
}

// configFromSuffix turns a parsed model-string suffix into a config.
func configFromSuffix(suffix Suffix) *Config {
	switch suffix.Mode {
	case "none":
		return &Config{Mode: "none"}
	case "auto":
		return &Config{Mode: "auto"}
	case "level":
		return &Config{Mode: "level", Level: suffix.Level}
	case "budget":
		return &Config{Mode: "budget", Budget: suffix.Budget}
	default:
		return nil
	}
}

// configFromMode turns a stored provider mode into a config. "on" is the
// reference's fixed budget and "off" is thinking disabled; every other mode is
// a level word, including "none", which the formats read as a disabled state.
func configFromMode(mode domain.ThinkingMode) *Config {
	switch mode {
	case domain.ThinkingOn:
		return &Config{Mode: "budget", Budget: OnBudget}
	case domain.ThinkingOff, domain.ThinkingMode("none"):
		return &Config{Mode: "none"}
	default:
		return &Config{Mode: "level", Level: string(mode)}
	}
}

// modelCapsFrom is the slice of the capability answer the formats read, plus
// the model's own level list, which the OpenAI format clamps against.
func modelCapsFrom(caps registry.CapabilitySet, call Call) modelCaps {
	mine := modelCaps{
		canDisable: caps.CanDisable,
		effort:     caps.EffortSupported,
		levels:     registry.ThinkingLevels(call.ProviderID, call.ModelID),
	}
	if caps.ThinkingRange != nil {
		mine.hasRange = true
		mine.rangeMin = caps.ThinkingRange.Min
		mine.rangeMax = caps.ThinkingRange.Max
	}
	return mine
}

// nativeOnlyFormats are the shapes only their own wire carries: an OpenAI-wire
// upstream cannot read Anthropic's or Gemini's thinking blocks, so a model
// declaring one of these on that wire falls back to the wire's own shape. The
// reference's NATIVE_ONLY_FORMATS, minus the kiro wire this port does not speak.
var nativeOnlyFormats = map[string]bool{
	"gemini-level":    true,
	"gemini-budget":   true,
	"claude-budget":   true,
	"claude-adaptive": true,
}

// resolveFormat names the shape the config is written in: the model's own
// declared format, unless the wire cannot carry it, and the wire's native
// format otherwise. It is the reference's resolveFormat.
func resolveFormat(wire string, caps registry.CapabilitySet) string {
	if format := caps.ThinkingFormat; format != "" {
		if !isOpenAIWire(wire) || !nativeOnlyFormats[format] {
			return format
		}
	}
	switch wire {
	case "openai", "openai-responses":
		return "openai"
	case "claude":
		return "claude-budget"
	default:
		return "openai"
	}
}

// isOpenAIWire reports whether a wire speaks the OpenAI family, the two targets
// the reference's NATIVE_ONLY guard covers.
func isOpenAIWire(wire string) bool {
	return wire == "openai" || wire == "openai-responses"
}
