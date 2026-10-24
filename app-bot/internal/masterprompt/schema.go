// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/schema.go
// @for       The mpv2 document shape and its strict decoder.
// @uses      bytes, encoding/json, errors, io.
// @reason    SPEC-BOT-001 §6.2 makes the document the only thing the builder may return, so the decoder rejects an invented section instead of losing it silently.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Version is the only document version this decoder accepts.
const Version = "mpv2"

// Document is one validated master prompt.
type Document struct {
	Version        string         `json:"version"`
	Modality       string         `json:"modality"`
	RoleFamily     string         `json:"role_family"`
	Role           string         `json:"role"`
	Task           string         `json:"task"`
	Context        Context        `json:"context"`
	Reasoning      string         `json:"reasoning"`
	StopConditions StopConditions `json:"stop_conditions"`
	Output         Output         `json:"output"`
	Seed           *int64         `json:"seed"`
}

// Context carries the visual facts of the request.
type Context struct {
	Subject     string      `json:"subject"`
	Action      string      `json:"action"`
	Environment string      `json:"environment"`
	Composition Composition `json:"composition"`
	Lighting    string      `json:"lighting"`
	Style       Style       `json:"style"`
	Details     []string    `json:"details"`
}

// Composition is the camera and frame choice.
type Composition struct {
	Shot        string `json:"shot"`
	Angle       string `json:"angle"`
	AspectRatio string `json:"aspect_ratio"`
}

// Style is the medium, palette, and mood.
type Style struct {
	Medium  string   `json:"medium"`
	Palette []string `json:"palette"`
	Mood    string   `json:"mood"`
}

// StopConditions states what must not appear, plus the hard limits.
type StopConditions struct {
	Negative      []string `json:"negative"`
	MaxCharacters int      `json:"max_characters"`
	AspectRatio   string   `json:"aspect_ratio"`
	DurationS     *int     `json:"duration_s"`
}

// Output states the deliverable.
type Output struct {
	Asset       string `json:"asset"`
	AspectRatio string `json:"aspect_ratio"`
	Count       int    `json:"count"`
	Format      string `json:"format"`
	DurationS   *int   `json:"duration_s"`
}

// Decode reads exactly one document, normalizes it, and validates it.
func Decode(raw []byte) (Document, error) {
	var doc Document
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, invalid("the document is not valid JSON for mpv2", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Document{}, invalid("the document carries data after the object", nil)
	}

	doc = normalizeDocument(doc)
	doc = remapRole(doc)
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}
