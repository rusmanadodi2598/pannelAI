// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/vocabulary.go
// @for       The closed vocabulary and budgets from SPEC-BOT-002.
// @uses      none.
// @reason    The compiler refuses an unknown enum instead of passing it upstream, so every accepted word is listed once here with its version.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

// VocabularyVersion bumps whenever a word below changes.
const VocabularyVersion = "v1"

const (
	budgetImage   = 3500
	budgetVideo   = 1800
	budgetPreview = 3500
	reasoningView = 160
	promptCap     = 2000
)

// familyDescriptor is what the compiler writes for a role family.
var familyDescriptor = map[string]string{
	"visual_designer":    "professional visual design",
	"photographer":       "professional photography",
	"illustrator":        "illustration",
	"motion_designer":    "motion design",
	"three_d_visualizer": "3d render",
}

// familyTitle is the neutral title used when the builder names a product.
var familyTitle = map[string]string{
	"visual_designer":    "visual designer",
	"photographer":       "photographer",
	"illustrator":        "illustrator",
	"motion_designer":    "motion designer",
	"three_d_visualizer": "3d artist",
}

// nonVisualWords marks a role that is not a visual specialist.
var nonVisualWords = []string{
	"growth strategist", "product manager", "marketing manager", "lawyer",
	"accountant", "sales", "copywriter", "recruiter", "engineer",
}

// productWords marks a role narrowed to one product or brand.
var productWords = []string{
	"iphone", "android", "our brand", "our product", "spotify", "nike",
	"adidas", "tokopedia", "shopee", "instagram", "tiktok",
}

var shots = wordSet("wide", "medium", "close_up", "macro", "aerial")

var angles = wordSet("eye_level", "low", "high", "top_down")

var aspects = wordSet("1:1", "3:4", "4:3", "9:16", "16:9")

var mediums = wordSet("photo", "cinematic photo", "illustration", "3d render", "anime", "watercolor")

var cameraMoves = wordSet("static", "pan_left", "pan_right", "push_in", "pull_out", "orbit")

var outputFormats = map[string][]string{
	"image": {"png", "jpeg", "webp"},
	"video": {"mp4"},
}

var imageSizes = map[string]string{
	"1:1": "1024x1024", "3:4": "1024x1536", "4:3": "1536x1024",
	"9:16": "1024x1792", "16:9": "1792x1024",
}

var videoSizes = map[string]string{
	"1:1": "1024x1024", "3:4": "1024x1536", "4:3": "1536x1024",
	"9:16": "1080x1920", "16:9": "1920x1080",
}

// baselineNegative is merged into every document that carries no negatives of
// its own, per SPEC-BOT-002 §4.5.
var baselineNegative = []string{
	"watermark", "text artifacts", "logo", "signature", "extra limbs",
	"deformed hands", "low resolution", "jpeg artifacts", "oversharpened",
}

// mediumAlias maps the words a user actually types onto the closed set.
var mediumAlias = map[string]string{
	"photo":           "photo",
	"foto":            "photo",
	"photography":     "photo",
	"cinematic":       "cinematic photo",
	"cinematic photo": "cinematic photo",
	"film":            "cinematic photo",
	"illustration":    "illustration",
	"ilustrasi":       "illustration",
	"3d":              "3d render",
	"3d render":       "3d render",
	"render":          "3d render",
	"anime":           "anime",
	"watercolor":      "watercolor",
	"cat air":         "watercolor",
	"akuarel":         "watercolor",
}

func wordSet(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func budgetFor(modality string) int {
	if modality == "video" {
		return budgetVideo
	}
	return budgetImage
}
