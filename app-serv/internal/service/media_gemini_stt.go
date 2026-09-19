// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_gemini_stt.go
// @for       The Gemini transcription adapter in the §7.10 media plane.
// @uses      internal/dataplane, internal/schema, encoding/base64,
//
//	encoding/json, strings.
//
// @reason    Gemini's transcription surface is the generateContent call its
//
//	speech surface already uses: the model is a path segment, the
//	credential rides in the query, the audio travels inline as base64,
//	and the answer is text rather than audio. Keeping it beside its TTS
//	sibling means both adapters share one URL shape and one gate.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// geminiTranscriptionPrompt is the reference's default instruction.
// generateContent is a general surface, so an unguided upload would be answered
// with commentary instead of only the transcript.
const geminiTranscriptionPrompt = "Generate a transcript of the speech. Return only the transcribed text, no commentary."

// geminiTranscriptionBody is the request Gemini's generateContent endpoint
// accepts for transcription: one text part and the audio inline.
//
// The field names are the reference's own spelling (`inline_data`, `mime_type`);
// Gemini's JSON mapping accepts that and the lowerCamelCase form alike, and the
// port keeps the reference's so the wire shape stays the one that was measured.
type geminiTranscriptionBody struct {
	Contents []geminiTranscriptionContent `json:"contents"`
}

// geminiTranscriptionContent is one turn of the conversation, which is how the
// endpoint frames a single request.
type geminiTranscriptionContent struct {
	Parts []geminiTranscriptionPart `json:"parts"`
}

// geminiTranscriptionPart is either the instruction or the audio, never both.
type geminiTranscriptionPart struct {
	Text       string             `json:"text,omitempty"`
	InlineData *geminiInlineAudio `json:"inline_data,omitempty"`
}

// geminiInlineAudio carries the uploaded bytes as base64 with their MIME type.
type geminiInlineAudio struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

// geminiTranscriptionRequest builds the provider's body and the URL its endpoint
// wants (`/{model}:generateContent`, with the credential query the target
// already carries).
func geminiTranscriptionRequest(form schema.TranscriptionForm, call MediaCall) (dataplane.MediaRequest, error) {
	target, err := dataplane.MediaPath(call.Target, "/"+call.UpstreamModel+":generateContent")
	if err != nil {
		return dataplane.MediaRequest{}, err
	}
	body, err := json.Marshal(geminiTranscriptionBody{
		Contents: []geminiTranscriptionContent{{Parts: []geminiTranscriptionPart{
			{Text: geminiTranscriptionInstruction(form)},
			{InlineData: &geminiInlineAudio{
				MIMEType: transcriptionContentType(form),
				Data:     base64.StdEncoding.EncodeToString(form.File),
			}},
		}}},
	})
	if err != nil {
		return dataplane.MediaRequest{}, dataplane.InternalError("the transcription request could not be built", err)
	}
	return dataplane.MediaRequest{Method: "POST", URL: target, Body: body}, nil
}

// geminiTranscriptionInstruction builds the prompt: the caller's own when it
// wrote one, the reference's default otherwise, with the language appended as a
// sentence so Gemini transcribes in it.
func geminiTranscriptionInstruction(form schema.TranscriptionForm) string {
	prompt := strings.TrimSpace(form.Prompt)
	if prompt == "" {
		prompt = geminiTranscriptionPrompt
	}
	if language := strings.TrimSpace(form.Language); language != "" {
		prompt += " Language: " + language + "."
	}
	return prompt
}

// geminiTranscriptionAnswer joins the text parts of the first candidate into the
// `{text}` shape the route returns.
//
// An answer with no candidate is an empty transcript rather than a refusal:
// silence transcribes to nothing, and the reference answers the same way. A
// malformed successful answer is still an internal error rather than a
// fabricated empty transcript.
func geminiTranscriptionAnswer(_ int, body []byte) ([]byte, error) {
	var answer struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, dataplane.InternalError("the transcription answer could not be read", err)
	}
	text := ""
	if len(answer.Candidates) > 0 {
		var builder strings.Builder
		for _, part := range answer.Candidates[0].Content.Parts {
			builder.WriteString(part.Text)
		}
		text = builder.String()
	}
	return transcriptionText(text)
}
