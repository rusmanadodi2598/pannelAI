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
//	business, only the decoded client body can answer it, so it is a
//	function here rather than part of the seam.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-19
package dataplane

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// VisionAugmenter is the seam an engine consults for an image-bearing request.
// Augment reports the order the request should walk and which entries in that order
// came from the adapter rather than from the request; the adapter's stored
// configuration, capability judgement and round-robin state all stay behind the seam,
// so the engine never reads them. It is handed every candidate because the question
// is "can anything this request could be served by see", not only the model it
// landed on: answering it from a combo's leading member can put the adapter in front
// of a member that reads images perfectly well. An empty adapted list means no
// augmentation happened, which is what a disabled adapter, a capable candidate or a
// failed read all answer.
type VisionAugmenter interface {
	Augment(ctx context.Context, candidates []string) (refs []string, adapted []string, err error)
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

// augmentForVision re-orders an image-bearing request's candidates through the seam
// and reports which of them came from the adapter. It is the engine's only contact
// with the policy, so the relay loop stays about the pipeline and not about the
// adapter.
//
// A seam failure fails open: the request is served in the order the request itself
// produced, which is the behaviour the adapter being disabled would have produced.
func (e *Engine) augmentForVision(ctx context.Context, in Request, refs []string) ([]string, []string) {
	if e.vision == nil || !carriesImages(in) {
		return refs, nil
	}
	order, adapted, err := e.vision.Augment(ctx, refs)
	if err != nil || len(adapted) == 0 {
		return refs, nil
	}
	return order, adapted
}

// containsRef reports whether one reference is in a list. The adapter's model list is
// bounded by its own configuration and searched once per served answer, so a set
// would cost more than it saves.
func containsRef(refs []string, ref string) bool {
	for _, candidate := range refs {
		if candidate == ref {
			return true
		}
	}
	return false
}
