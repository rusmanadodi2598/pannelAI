// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/media_transcription.go
// @for       The transcription contract of SPEC-API-001 §7.10.
// @uses      bytes, encoding/json, io, mime/multipart, net/http, strings,
//
//	internal/domain.
//
// @reason    A transcription is the one media request that is not JSON: it
//
//	arrives as multipart, so it is decoded and bounded here rather than
//	in the handler, and the bound is what keeps an oversized upload off
//	the filesystem. The speech and voice contracts live in media_audio.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// MaxTranscriptionBytes bounds a transcription upload. OpenAI refuses an audio
// file above 25 MB, so the gateway refuses it at the edge rather than streaming
// it to an upstream that would.
const MaxTranscriptionBytes = 25 << 20

// TranscriptionForm is the decoded multipart body of
// POST /api/v1/audio/transcriptions.
type TranscriptionForm struct {
	Model          string
	Language       string
	Prompt         string
	ResponseFormat string
	Temperature    string
	Filename       string
	File           []byte
}

// ReadTranscriptionForm parses the multipart upload, bounding both the request
// and the file. The caller must have wrapped the body with http.MaxBytesReader
// first: ParseMultipartForm spills to disk past its memory budget, so the edge
// cap is what keeps an oversized upload off the filesystem.
func ReadTranscriptionForm(r *http.Request) (TranscriptionForm, error) {
	if r.Body == nil {
		return TranscriptionForm{}, domain.NewValidationError("request body is required")
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return TranscriptionForm{}, domain.NewValidationError("invalid multipart body: " + err.Error())
	}
	form := TranscriptionForm{
		Model:          strings.TrimSpace(r.FormValue("model")),
		Language:       strings.TrimSpace(r.FormValue("language")),
		Prompt:         strings.TrimSpace(r.FormValue("prompt")),
		ResponseFormat: strings.TrimSpace(r.FormValue("response_format")),
		Temperature:    strings.TrimSpace(r.FormValue("temperature")),
	}
	if form.Model == "" {
		return TranscriptionForm{}, domain.NewValidationError("field model is required")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return TranscriptionForm{}, domain.NewValidationError("field file is required")
	}
	defer func() {
		// reason: the bytes are already read into memory below, so a close
		// error here reports nothing a caller could act on.
		_ = file.Close()
	}()
	content, err := readBounded(file, MaxTranscriptionBytes)
	if err != nil {
		return TranscriptionForm{}, err
	}
	form.File = content
	form.Filename = safeUploadName(header)
	return form, nil
}

// readBounded reads at most limit bytes, refusing one byte more rather than
// truncating an upload.
func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, domain.NewValidationError("the uploaded file could not be read")
	}
	if int64(len(content)) > limit {
		return nil, domain.NewValidationError("the uploaded file is too large")
	}
	if len(content) == 0 {
		return nil, domain.NewValidationError("the uploaded file is empty")
	}
	return content, nil
}

// safeUploadName keeps the client's filename for the upstream part, reduced to
// its base name so a crafted path cannot steer where an upstream stores it.
func safeUploadName(header *multipart.FileHeader) string {
	if header == nil {
		return "audio"
	}
	name := strings.TrimSpace(header.Filename)
	if name == "" {
		return "audio"
	}
	if base := name[strings.LastIndexAny(name, `/\`)+1:]; base != "" {
		name = base
	}
	if name == "." || name == ".." {
		return "audio"
	}
	return name
}

// TranscriptionJSON reports whether an upstream answer is JSON, which is what
// decides the content type the route answers with: a transcription upstream may
// answer with plain text when the caller asked for `text`.
func TranscriptionJSON(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	return json.Valid(trimmed)
}
