// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_transcription.go
// @for       Format-specific transcription request and answer handling for the
//
//	§7.10 media plane.
//
// @uses      internal/dataplane, internal/registry, internal/schema, encoding/json,
//
//	mime, strings.
//
// @reason    OpenAI-compatible transcription is multipart, while Deepgram's
//
//	adapter accepts the uploaded audio bytes and returns a nested transcript.
//	Keeping both shapes here lets the shared media pipeline retain one
//	selection, health, egress, and accounting path without pretending every
//	provider speaks the same wire.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/json"
	"mime"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// transcriptionQuery selects the format-specific query without changing the
// caller's query for OpenAI-compatible providers. Provider lookup resolves ids
// and aliases before the call, so `dg/nova-3` gets the same Deepgram shape as
// `deepgram/nova-3`.
func (s *MediaCallService) transcriptionQuery(form schema.TranscriptionForm, query map[string]string) map[string]string {
	providerID, upstreamModel, err := splitMediaModel(form.Model)
	if err != nil {
		return query
	}
	entry, ok := s.index.Provider(providerID)
	if !ok {
		return query
	}
	media, ok := entry.Media.For(registry.MediaSTT)
	if !ok || !strings.EqualFold(strings.TrimSpace(media.Format), "deepgram") {
		return query
	}
	return deepgramQuery(form, upstreamModel)
}

// deepgramQuery builds the provider's declared request parameters. Query values
// are handed to MediaTarget, which escapes them through net/url before dialing.
func deepgramQuery(form schema.TranscriptionForm, model string) map[string]string {
	query := map[string]string{
		"model":        model,
		"smart_format": "true",
		"punctuate":    "true",
	}
	if language := strings.TrimSpace(form.Language); language != "" {
		query["language"] = language
	} else {
		query["detect_language"] = "true"
	}
	return query
}

// deepgramRequest returns the raw audio request Deepgram expects. The MIME
// value is derived by transcriptionContentType, never copied as an arbitrary
// multipart header value into the outbound request.
func deepgramRequest(form schema.TranscriptionForm) dataplane.MediaRequest {
	return dataplane.MediaRequest{
		Method: "POST", Body: form.File,
		Headers: map[string]string{"Content-Type": transcriptionContentType(form)},
	}
}

// transcriptionContentType accepts a parsed audio MIME type or derives one from
// the sanitized filename. Unknown and non-audio values fall back to the safe
// octet-stream type, matching the reference's behavior for an unknown suffix.
func transcriptionContentType(form schema.TranscriptionForm) string {
	if mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(form.ContentType)); err == nil &&
		strings.HasPrefix(strings.ToLower(mediaType), "audio/") {
		return strings.ToLower(mediaType)
	}
	name := strings.ToLower(strings.TrimSpace(form.Filename))
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		if contentType, ok := audioContentTypes[name[index+1:]]; ok {
			return contentType
		}
	}
	return "application/octet-stream"
}

var audioContentTypes = map[string]string{
	"aac":  "audio/aac",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
	"mp3":  "audio/mpeg",
	"mp4":  "audio/mp4",
	"ogg":  "audio/ogg",
	"opus": "audio/opus",
	"wav":  "audio/wav",
	"webm": "audio/webm",
}

// transcriptionRequest builds the request the format's upstream expects. The
// OpenAI shape is multipart, Deepgram takes the raw uploaded bytes, and Gemini's
// transcription surface is one generateContent call with the audio inline.
func transcriptionRequest(form schema.TranscriptionForm, call MediaCall) (dataplane.MediaRequest, error) {
	switch mediaFormat(call.Media) {
	case "deepgram":
		return deepgramRequest(form), nil
	case "gemini-stt":
		return geminiTranscriptionRequest(form, call)
	default:
		body, contentType, err := transcriptionBody(form, call.UpstreamModel)
		if err != nil {
			return dataplane.MediaRequest{}, err
		}
		return dataplane.MediaRequest{
			Method: "POST", Body: body,
			// The target carries a JSON content type; a multipart body must
			// replace it, boundary included.
			Headers: map[string]string{"Content-Type": contentType},
		}, nil
	}
}

// transcriptionReader reports how a provider's transcription answer is read into
// the OpenAI shape: Deepgram nests its transcript, Gemini spreads it over parts,
// the rest answer it already.
func transcriptionReader(media registry.MediaConfig) mediaAnswerReader {
	switch mediaFormat(media) {
	case "deepgram":
		return func(_ int, body []byte) ([]byte, error) { return deepgramTranscriptionAnswer(body) }
	case "gemini-stt":
		return geminiTranscriptionAnswer
	default:
		return nil
	}
}

// deepgramTranscriptionAnswer keeps the OpenAI response shape for every client
// while adapting Deepgram's nested response. A malformed successful response is
// an internal error rather than a fabricated empty transcript.
func deepgramTranscriptionAnswer(body []byte) ([]byte, error) {
	var answer struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, dataplane.InternalError("the transcription answer could not be read", err)
	}
	text := ""
	if len(answer.Results.Channels) > 0 && len(answer.Results.Channels[0].Alternatives) > 0 {
		text = answer.Results.Channels[0].Alternatives[0].Transcript
	}
	return transcriptionText(text)
}

// transcriptionText is the `{text}` answer the transcription route returns
// whatever the provider's own shape was, so the adapters cannot drift apart.
func transcriptionText(text string) ([]byte, error) {
	encoded, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	if err != nil {
		return nil, dataplane.InternalError("the transcription answer could not be built", err)
	}
	return encoded, nil
}
