// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/vision.go
// @for       The vision augmentation seam and the image-content detection the
//
//	engine consults before relaying (SPEC-API-001 §7.8).
//
// @uses      internal/schema, context.
// @reason    Whether an image-bearing request should try other models first is a
//
//	policy of the vision adapter configuration, not of the pipeline:
//	declaring it as a one-method seam keeps the engine ignorant of the
//	adapter's storage and rotation, the way the transport is ignorant
//	of any provider's quirk. The detection half is the pipeline's own
//	business — only the decoded client body can answer it — so it is a
//	function here rather than part of the seam.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// VisionAugmenter is the seam an engine consults for an image-bearing request.
//
// Augment reports the model references to try BEFORE the request's own model,
// and whether augmentation applies at all. Everything §7.8 layers on top — the
// stored configuration, the capability judgement, the round-robin state — lives
// behind the seam, so the engine neither reads the adapter's configuration nor
// knows whether one is enabled.
type VisionAugmenter interface {
	Augment(ctx context.Context, providerID, modelID string) (refs []string, applies bool, err error)
}

// carriesImages reports whether the decoded client body carries image content.
//
// Both wire formats are checked because the client's format and the upstream's
// are independent: an Anthropic client may be served by an OpenAI provider, and
// the question "does this request carry an image" precedes translation.
func carriesImages(in Request) bool {
	if in.Chat != nil {
		for _, message := range in.Chat.Messages {
			if message.Content.HasImage() {
				return true
			}
		}
	}
	if in.Messages != nil {
		for _, message := range in.Messages.Messages {
			for _, block := range message.Content {
				if block.Type == schema.BlockImage {
					return true
				}
			}
		}
	}
	return false
}

// augmentForVision prepends the adapter's model references when the request
// carries images, and reports how many of the list's leading members are
// adapter models. It is the engine's only contact with the §7.8 policy, so the
// relay loop stays about the pipeline and not about the adapter.
//
// A seam failure fails open: the request is served un-augmented, which is the
// behaviour the adapter being disabled would have produced.
func (e *Engine) augmentForVision(ctx context.Context, in Request, resolution Resolution, refs []string) ([]string, int) {
	if e.vision == nil || !carriesImages(in) {
		return refs, 0
	}
	adapterRefs, applies, err := e.vision.Augment(ctx, resolution.Provider.ID, resolution.ModelID)
	if err != nil || !applies || len(adapterRefs) == 0 {
		return refs, 0
	}
	return append(adapterRefs, refs...), len(adapterRefs)
}
