// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/support_test.go
// @for       Document fixtures shared by the masterprompt tests.
// @uses      testing, encoding/json, os, path/filepath.
// @reason    Three suites decode the same two documents, so the fixtures live here instead of being rebuilt per file.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

func imageDocument(t *testing.T) map[string]any {
	t.Helper()
	raw := []byte(`{
		"version": "mpv2",
		"modality": "image",
		"role_family": "visual_designer",
		"role": "senior visual designer with campaign experience",
		"task": "create a hero image of an orange cat riding a skateboard through Jakarta",
		"context": {
			"subject": "an orange cat riding a skateboard",
			"action": "rolling through a busy street",
			"environment": "Jakarta, late afternoon, street vendors",
			"composition": {"shot": "medium", "angle": "eye_level", "aspect_ratio": "16:9"},
			"lighting": "warm golden hour, soft shadows",
			"style": {"medium": "cinematic photo", "palette": ["warm orange", "teal shadow"], "mood": "playful"},
			"details": ["motion blur in the wheels", "street vendors in the background"]
		},
		"reasoning": "The subject is playful and local, so a warm street scene with motion reads faster than a studio setup. A medium shot keeps the cat and the board legible at phone size.",
		"stop_conditions": {
			"negative": ["watermark", "text artifacts", "extra limbs"],
			"max_characters": 3500,
			"aspect_ratio": "16:9",
			"duration_s": null
		},
		"output": {"asset": "image", "aspect_ratio": "16:9", "count": 1, "format": "png", "duration_s": null},
		"seed": null
	}`)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal the image fixture: %v", err)
	}
	return doc
}

func videoDocument(t *testing.T) map[string]any {
	t.Helper()
	raw := []byte(`{
		"version": "mpv2",
		"modality": "video",
		"role_family": "motion_designer",
		"role": "senior motion designer",
		"task": "create a product reveal video of a ceramic teapot",
		"context": {
			"subject": "a ceramic teapot on a wooden table",
			"action": "slow reveal",
			"environment": "studio, dark backdrop",
			"composition": {"shot": "close_up", "angle": "low", "aspect_ratio": "9:16"},
			"lighting": "soft rim light",
			"style": {"medium": "3d render", "palette": ["slate grey"], "mood": "calm"},
			"details": ["push_in", "steam rising"]
		},
		"reasoning": "The teapot reads as a single hero object, so a close shot with a low angle and a slow push in keeps the rim light on the ceramic. Six seconds is enough for one reveal.",
		"stop_conditions": {
			"negative": ["flicker"],
			"max_characters": 1800,
			"aspect_ratio": "9:16",
			"duration_s": 6
		},
		"output": {"asset": "video", "aspect_ratio": "9:16", "count": 1, "format": "mp4", "duration_s": 6},
		"seed": 42
	}`)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal the video fixture: %v", err)
	}
	return doc
}

func encode(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal the fixture: %v", err)
	}
	return raw
}

func decodeImage(t *testing.T) masterprompt.Document {
	t.Helper()
	doc, err := masterprompt.Decode(encode(t, imageDocument(t)))
	if err != nil {
		t.Fatalf("Decode() image fixture error = %v, want nil", err)
	}
	return doc
}

func decodeVideo(t *testing.T) masterprompt.Document {
	t.Helper()
	doc, err := masterprompt.Decode(encode(t, videoDocument(t)))
	if err != nil {
		t.Fatalf("Decode() video fixture error = %v, want nil", err)
	}
	return doc
}

func golden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(raw)
}
