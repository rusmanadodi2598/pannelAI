// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_gemini_tts.go
// @for       The Gemini speech adapter in the §7.10 media plane.
// @uses      internal/dataplane, internal/schema, encoding/base64, encoding/binary,
//
//	encoding/json, strings.
//
// @reason    Gemini's speech endpoint is its generateContent surface: the model
//
//	is a path segment, the credential rides in the query, the request is
//	a prompt, and the answer is base64 PCM rather than a container. The
//	adapter wraps that PCM in a WAV header so the route's `wav` label is
//	true, which is the reference's own conversion.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// geminiDefaultVoice is the reference's default voice name; Gemini's catalog is
// a fixed list rather than an API, so the default is data rather than a lookup.
const geminiDefaultVoice = "Kore"

// Gemini answers 16-bit mono PCM at 24 kHz, which the WAV header below declares.
const (
	geminiSampleRate    = 24000
	geminiChannels      = 1
	geminiBitsPerSample = 16
)

// geminiSpeechBody is the request Gemini's generateContent endpoint accepts for
// speech.
type geminiSpeechBody struct {
	Contents         []geminiSpeechContent  `json:"contents"`
	GenerationConfig geminiSpeechGeneration `json:"generationConfig"`
}

// geminiSpeechContent keeps the nested prompt shape typed (AGENTS.md §1.4).
type geminiSpeechContent struct {
	Parts []geminiSpeechPart `json:"parts"`
}

// geminiSpeechPart is one part of the prompt.
type geminiSpeechPart struct {
	Text string `json:"text"`
}

// geminiSpeechGeneration asks for audio and names the voice.
type geminiSpeechGeneration struct {
	ResponseModalities []string           `json:"responseModalities"`
	SpeechConfig       geminiSpeechConfig `json:"speechConfig"`
}

// geminiSpeechConfig nests the voice selection.
type geminiSpeechConfig struct {
	VoiceConfig geminiVoiceConfig `json:"voiceConfig"`
}

// geminiVoiceConfig nests the prebuilt voice the request names.
type geminiVoiceConfig struct {
	PrebuiltVoiceConfig geminiPrebuiltVoice `json:"prebuiltVoiceConfig"`
}

// geminiPrebuiltVoice carries the voice name.
type geminiPrebuiltVoice struct {
	VoiceName string `json:"voiceName"`
}

// geminiSpeechPayload builds the provider's body and the URL its endpoint wants
// (`/{model}:generateContent`, with the credential query the target already
// carries).
func geminiSpeechPayload(req schema.SpeechRequest, call MediaCall) (speechPayload, error) {
	target, err := dataplane.MediaPath(call.Target, "/"+call.UpstreamModel+":generateContent")
	if err != nil {
		return speechPayload{}, err
	}
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = geminiDefaultVoice
	}
	body, err := json.Marshal(geminiSpeechBody{
		Contents: []geminiSpeechContent{{Parts: []geminiSpeechPart{{Text: geminiSpeechPrompt(req)}}}},
		GenerationConfig: geminiSpeechGeneration{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: geminiSpeechConfig{VoiceConfig: geminiVoiceConfig{
				PrebuiltVoiceConfig: geminiPrebuiltVoice{VoiceName: voice},
			}},
		},
	})
	return speechPayload{body: body, target: target}, err
}

// geminiSpeechPrompt builds the TTS prompt. Gemini speaks a plain prompt in its
// default voice, so the reference prefixes the text with a directive unless the
// caller already wrote one (`Say in Indonesian: …`).
func geminiSpeechPrompt(req schema.SpeechRequest) string {
	text := req.Input
	if strings.Contains(text, ": ") {
		return text
	}
	if language := strings.TrimSpace(req.Language); language != "" {
		return "Say in " + language + ": " + text
	}
	return "Say: " + text
}

// geminiSpeechCandidate is one answer candidate: its parts, and the reason the
// model stopped when it produced none.
type geminiSpeechCandidate struct {
	Content struct {
		Parts []struct {
			InlineData struct {
				Data string `json:"data"`
			} `json:"inlineData"`
		} `json:"parts"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

// geminiSpeechAnswer decodes the base64 PCM an answer carries and wraps it in a
// WAV header, so the `wav` label the route reports matches the bytes.
//
// A 200 with no audio part is a failure the client would otherwise receive as an
// empty file: Gemini reports it as a finish reason or a prompt block, and that
// reason travels with the refusal.
func geminiSpeechAnswer(status int, body []byte) ([]byte, error) {
	var answer struct {
		Candidates     []geminiSpeechCandidate `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, dataplane.InternalError("the speech answer could not be read", err)
	}
	pcm := ""
	if len(answer.Candidates) > 0 {
		for _, part := range answer.Candidates[0].Content.Parts {
			if part.InlineData.Data != "" {
				pcm = part.InlineData.Data
				break
			}
		}
	}
	if pcm == "" {
		return nil, dataplane.UpstreamRejected(status, geminiNoAudioReason(answer.Candidates, answer.PromptFeedback.BlockReason))
	}
	audio, err := base64.StdEncoding.DecodeString(pcm)
	if err != nil {
		return nil, dataplane.UpstreamRejected(status, "the speech upstream returned an unreadable audio payload")
	}
	return pcmToWAV(audio), nil
}

// geminiNoAudioReason names why the answer carried no audio, so the log says
// which limit the request hit rather than only that it failed.
func geminiNoAudioReason(candidates []geminiSpeechCandidate, blockReason string) string {
	reason := strings.TrimSpace(blockReason)
	if reason == "" && len(candidates) > 0 {
		reason = strings.TrimSpace(candidates[0].FinishReason)
	}
	if reason == "" {
		return "the speech upstream returned no audio"
	}
	return "the speech upstream returned no audio (" + reason + ")"
}

// pcmToWAV prefixes raw PCM with the 44-byte RIFF header that declares the
// format Gemini answers in, which is the reference's own conversion.
func pcmToWAV(pcm []byte) []byte {
	const headerBytes = 44
	header := make([]byte, headerBytes)
	byteRate := geminiSampleRate * geminiChannels * geminiBitsPerSample / 8
	blockAlign := geminiChannels * geminiBitsPerSample / 8

	copy(header[0:4], "RIFF")
	// The RIFF size field counts the file minus the eight bytes of `RIFF`+size.
	binary.LittleEndian.PutUint32(header[4:8], uint32(headerBytes-8+len(pcm)))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], geminiChannels)
	binary.LittleEndian.PutUint32(header[24:28], geminiSampleRate)
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], geminiBitsPerSample)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(len(pcm)))
	return append(header, pcm...)
}
