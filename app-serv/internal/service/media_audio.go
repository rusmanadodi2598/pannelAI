// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_audio.go
// @for       The speech, transcription, and voice use cases of SPEC-API-001 §7.10.
// @uses      internal/dataplane, internal/domain, internal/registry, internal/schema,
//
//	bytes, context, encoding/json, mime/multipart, strings.
//
// @reason    Each method here is one route's payload, built over the shared
//
//	pipeline in media_call.go: the reference's OpenAI-compatible
//	transcription path forwards the caller's optional fields and
//	returns the upstream body, and its speech path sends a fixed
//	OpenAI payload — both are request shaping, which is this layer's
//	job and not the handler's.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// Speech performs one text-to-speech call and answers with whatever the
// upstream returned: the audio bytes, which the handler renders as the caller
// asked for.
func (s *MediaCallService) Speech(ctx context.Context, req schema.SpeechRequest, query map[string]string, keyID string) (dataplane.MediaResponse, MediaCall, error) {
	call, err := s.prepareForCall(ctx, req.Model, domain.MediaKindTTS, query, keyID)
	if err != nil {
		return dataplane.MediaResponse{}, MediaCall{}, err
	}
	var body []byte
	if strings.EqualFold(strings.TrimSpace(call.Media.Format), "nvidia-tts") {
		body, err = json.Marshal(nvidiaSpeechRequest(req, call.UpstreamModel))
	} else {
		body, err = json.Marshal(schema.SpeechBody{
			Model:          call.UpstreamModel,
			Input:          req.Input,
			Voice:          schema.SpeechVoice(req),
			ResponseFormat: strings.TrimSpace(req.ResponseFormat),
			Speed:          req.Speed,
		})
	}
	if err != nil {
		return dataplane.MediaResponse{}, call, dataplane.InternalError("the speech request could not be built", err)
	}
	answer, err := s.Perform(ctx, call, dataplane.MediaRequest{Method: "POST", Body: body}, keyID)
	return answer, call, err
}

// Transcribe performs one speech-to-text call, forwarding the caller's optional
// fields the way the reference's OpenAI-compatible path does.
func (s *MediaCallService) Transcribe(ctx context.Context, form schema.TranscriptionForm, query map[string]string, keyID string) (dataplane.MediaResponse, MediaCall, error) {
	callQuery := s.transcriptionQuery(form, query)
	call, err := s.prepareForCall(ctx, form.Model, domain.MediaKindSTT, callQuery, keyID)
	if err != nil {
		return dataplane.MediaResponse{}, MediaCall{}, err
	}
	var request dataplane.MediaRequest
	if strings.EqualFold(strings.TrimSpace(call.Media.Format), "deepgram") {
		request = deepgramRequest(form)
	} else {
		body, contentType, buildErr := transcriptionBody(form, call.UpstreamModel)
		if buildErr != nil {
			return dataplane.MediaResponse{}, call, buildErr
		}
		request = dataplane.MediaRequest{
			Method: "POST", Body: body,
			// The target carries a JSON content type; a multipart body must
			// replace it, boundary included.
			Headers: map[string]string{"Content-Type": contentType},
		}
	}
	answer, err := s.Perform(ctx, call, request, keyID)
	if err != nil {
		return answer, call, err
	}
	if normalized, normalizeErr := normalizeTranscription(call.Media, answer.Body); normalizeErr != nil {
		return dataplane.MediaResponse{}, call, normalizeErr
	} else {
		answer.Body = normalized
	}
	return answer, call, nil
}

// Voices returns the speech catalog the provider declares. §7.10's route exists
// so a client can pick a voice, and the registry is the only source that does
// not require calling an upstream to answer: a provider that declares none is
// refused by name rather than answered with its model list.
func (s *MediaCallService) Voices(providerID string) ([]registry.MediaVoice, error) {
	trimmed := strings.TrimSpace(providerID)
	entry, ok := s.index.Provider(trimmed)
	if !ok {
		return nil, dataplane.ErrNotFound("provider " + trimmed + " is not in the registry")
	}
	media, ok := entry.Media.For(registry.MediaTTS)
	if !ok {
		return nil, dataplane.ProviderNotRoutable("provider " + entry.ID + " does not offer tts")
	}
	if len(media.Voices) == 0 {
		return nil, dataplane.ProviderNotRoutable(
			"provider " + entry.ID + " declares no voice catalog; its voices are fetched live in the reference")
	}
	return media.Voices, nil
}

// transcriptionBody builds the multipart body an OpenAI-compatible
// transcription endpoint expects, returning the body and its content type.
func transcriptionBody(form schema.TranscriptionForm, upstreamModel string) ([]byte, string, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	part, err := writer.CreateFormFile("file", form.Filename)
	if err != nil {
		return nil, "", dataplane.InternalError("the transcription request could not be built", err)
	}
	if _, err := part.Write(form.File); err != nil {
		return nil, "", dataplane.InternalError("the transcription request could not be built", err)
	}
	for _, field := range []struct{ name, value string }{
		{name: "model", value: upstreamModel},
		{name: "language", value: form.Language},
		{name: "prompt", value: form.Prompt},
		{name: "response_format", value: form.ResponseFormat},
		{name: "temperature", value: form.Temperature},
	} {
		if field.value == "" {
			continue
		}
		if err := writer.WriteField(field.name, field.value); err != nil {
			return nil, "", dataplane.InternalError("the transcription request could not be built", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", dataplane.InternalError("the transcription request could not be built", err)
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}
