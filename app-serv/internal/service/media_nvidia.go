// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_nvidia.go
// @for       The NVIDIA NIM speech adapter in the §7.10 media plane.
// @uses      internal/registry, internal/schema, strings.
// @reason    NVIDIA's TTS endpoint accepts a small provider-specific JSON shape
//
//	and returns WAV bytes, while the surrounding route keeps the OpenAI-ish
//	speech response. Keeping its voice default, request body, and output
//	format together prevents the generic OpenAI builder from sending
//	`input` as a string or labeling WAV bytes as mp3.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// nvidiaSpeechBody is the request NVIDIA NIM's generic TTS adapter accepts.
type nvidiaSpeechBody struct {
	Input nvidiaSpeechInput `json:"input"`
	Voice string            `json:"voice"`
	Model string            `json:"model"`
}

// nvidiaSpeechInput keeps the provider's nested text shape typed at the JSON
// boundary, rather than assembling an untyped map (AGENTS.md §1.4).
type nvidiaSpeechInput struct {
	Text string `json:"text"`
}

// nvidiaSpeechRequest builds the provider-specific request. The upstream does
// not consume OpenAI's response_format or speed fields; its answer is WAV.
func nvidiaSpeechRequest(req schema.SpeechRequest, model string) nvidiaSpeechBody {
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = "default"
	}
	return nvidiaSpeechBody{
		Input: nvidiaSpeechInput{Text: req.Input}, Voice: voice, Model: model,
	}
}

// SpeechOutputFormat maps the provider's response bytes to the route's output
// label. NVIDIA's adapter always returns WAV; the OpenAI-shaped path follows the
// client's requested format or its mp3 default.
func (c MediaCall) SpeechOutputFormat(req schema.SpeechRequest) string {
	if strings.EqualFold(strings.TrimSpace(c.Media.Format), "nvidia-tts") {
		return "wav"
	}
	return schema.SpeechFormat(req)
}
