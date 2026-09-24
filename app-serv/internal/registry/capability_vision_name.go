// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_vision_name.go
// @for       The reference's last-resort vision guess: a model id that names its
// //
//
//	own modality.
//
// @uses      regexp, strings.
// @reason    The reference refines every capability answer with
//
//	`looksLikeVisionModel` (capabilities.js:520), which reads the model id
//	itself: vendors put the modality in the name ("qwen3-vl-plus",
//	"glm-4.6v", "deepseek-v4-flash-vision-exp"), so a model nobody has
//	catalogued yet still accepts images instead of silently dropping them.
//	It only ever turns vision ON, which is why it runs after the tables
//	rather than before: a table that answered false must not be overridden
//	by a name that happens to contain "omni".
//
//	It is ported rather than skipped because the pinned corpus proves it is
//	load-bearing: codebuddy-intl's glm-5v-turbo answers true only through
//	this layer, and the corpus test fails by name when it is missing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package registry

import (
	"regexp"
	"strings"
)

// visionSeparators is the character class the reference treats as a word
// boundary inside a model id.
const visionSeparators = `[-_/:.]`

// notVisionName matches the ids that name something other than image input.
// Image and video generation, embedding, moderation, and speech models all
// carry words that would otherwise look like a modality, so they are excluded
// before the positive test runs.
var notVisionName = regexp.MustCompile(`(?i)(^|` + visionSeparators + `)(image|img)($|` + visionSeparators + `)|` +
	`stable-image|gen[0-9]_image|nanobanana|imagine|` +
	`t2v|i2v|flux|dall|sdxl|diffusion|` +
	`embed|rerank|guard|moderation|` +
	`tts|stt|whisper|voice|speech|audio`)

// visionName matches the ids that do name image input: the explicit modality
// words, the `<digit>.<digit>v` suffix vendors use for a vision variant, the
// `glm-<n>v` family, and the open-weight vision families.
//
// The dotted-version requirement on the `v` suffix is the reference's own: it
// keeps the never-shipped `gpt-4v` from matching while `glm-4.6v` does.
var visionName = regexp.MustCompile(`(?i)(^|` + visionSeparators + `)(vision|vl|vlm|multimodal|omni|visual)($|` + visionSeparators + `)|` +
	`[0-9]\.[0-9]+v($|` + visionSeparators + `)|` +
	`(^|` + visionSeparators + `)glm-[0-9]+v($|` + visionSeparators + `)|` +
	`(^|[-_/:.])(llava|pixtral|internvl|cogvlm|minicpm-v|moondream|idefics|fuyu)`)

// looksLikeVisionModel reports whether a model id names its own vision
// capability. It is a name signal only, and it never answers false for a model
// the tables already declared capable: the caller applies it last and only in
// the true direction.
func looksLikeVisionModel(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return false
	}
	if notVisionName.MatchString(id) {
		return false
	}
	return visionName.MatchString(id)
}
