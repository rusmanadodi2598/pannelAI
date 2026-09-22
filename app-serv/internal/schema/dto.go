// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/dto.go
// @for       Gateway key request/response contracts and pagination decoding.
// @uses      go-playground/validator/v10 (struct-tag validation, the documented
//
//	exception to the stdlib-first rule in AGENTS.md "Stack").
//
// @reason    AGENTS.md §2.4 CDD requires contracts to be typed structs with
//
//	validation tags written before handlers, and SPEC-API-001 §7.3
//	fixes the gateway key wire shapes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-16
package schema

import (
	"errors"
	"fmt"

	val "github.com/go-playground/validator/v10"

	"io"
	"net/http"
	"strconv"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// MaxPerPage caps any list request, mirroring SPEC-API-001 §4 ("max 100").
const MaxPerPage = 100

// DefaultPerPage is used when the caller sends no per_page.
const DefaultPerPage = 25

// MaxBodyBytes bounds request bodies so a huge payload fails at the read
// boundary rather than consuming memory during decode.
const MaxBodyBytes = 8 << 20

// Page is the meta block every list endpoint returns (SPEC-API-001 §4).
type Page struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

// DecodePage reads page/per_page query params and enforces the documented
// ranges: page >= 1, 1 <= per_page <= MaxPerPage. An out-of-range value is a
// VALIDATION_ERROR rather than a silent clamp, so a caller can tell the page it
// asked for from the page it received (draft 010 F6, owner decision D3).
func DecodePage(r *http.Request) (page, perPage int, err error) {
	page = 1
	if v := r.URL.Query().Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, domain.NewValidationError("page must be an integer")
		}
		if page < 1 {
			return 0, 0, domain.NewValidationError("page must be at least 1")
		}
	}

	perPage = DefaultPerPage
	if v := r.URL.Query().Get("per_page"); v != "" {
		perPage, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, domain.NewValidationError("per_page must be an integer")
		}
		if perPage < 1 {
			return 0, 0, domain.NewValidationError("per_page must be at least 1")
		}
		if perPage > MaxPerPage {
			return 0, 0, domain.NewValidationError("per_page must be at most 100")
		}
	}

	return page, perPage, nil
}

// CreateGatewayKeyRequest is the body of POST /api/v1/gateway-keys (§7.3).
type CreateGatewayKeyRequest struct {
	Name string `json:"name" validate:"required,min=1,max=120"`
}

// UpdateGatewayKeyRequest is the body of PATCH /api/v1/gateway-keys/{id} (§7.3).
// Pointers keep "omitted" distinct from "set to empty" for a partial update.
type UpdateGatewayKeyRequest struct {
	Name   *string `json:"name,omitempty" validate:"omitempty,min=1,max=120"`
	Status *string `json:"status,omitempty" validate:"omitempty,oneof=active disabled"`
}

// GatewayKeyResponse is the list/detail shape; the secret is never included
// after creation (SPEC-API-001 §4 key masking). PlaintextKey is set only on
// the create response, and every timestamp is RFC3339 UTC (§4).
type GatewayKeyResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	KeyHint      string  `json:"key_hint"`
	Status       string  `json:"status"`
	LastUsedAt   *string `json:"last_used_at,omitempty"`
	RequestCount int64   `json:"request_count"`
	CreatedAt    string  `json:"created_at"`
	RevokedAt    *string `json:"revoked_at,omitempty"`
	PlaintextKey string  `json:"plaintext_key,omitempty"`
}

// GatewayKeyList wraps a page of keys with the pagination meta block.
type GatewayKeyList struct {
	Data []GatewayKeyResponse `json:"data"`
	Meta Page                 `json:"meta"`
}

// DecodeJSON reads a bounded body and unmarshals it into dst, which must be a
// concrete typed struct. Unknown fields are rejected so a typo'd key is never
// silently dropped (contract discipline, AGENTS.md §2.4).
func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return domain.NewValidationError("request body is required")
	}
	dec := jsonDecoder(io.LimitReader(r.Body, MaxBodyBytes))
	if err := dec.Decode(dst); err != nil {
		return domain.NewValidationError("invalid request body: " + cleanDecodeError(err))
	}
	return nil
}

// cleanDecodeError keeps a bounded, English tail of a codec error so the
// envelope never echoes a large or driver-specific message.
func cleanDecodeError(err error) string {
	msg := err.Error()
	if i := indexLast(msg, ":"); i > 0 && i < len(msg)-1 {
		tail := msg[i+1:]
		if len(tail) <= 128 {
			return tail
		}
	}
	if len(msg) > 128 {
		return msg[len(msg)-128:]
	}
	return msg
}

func indexLast(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ValidateStruct runs the struct-tag rules attached to v. The single engine
// is constructed once in validator.go and reused for every request.
func ValidateStruct(v any) error {
	if err := engine().Struct(v); err != nil {
		return domain.NewValidationError(firstFieldError(err))
	}
	switch request := v.(type) {
	case ChatRequest:
		return ValidateChatRequest(request)
	case *ChatRequest:
		if request != nil {
			return ValidateChatRequest(*request)
		}
	}
	return nil
}

// firstFieldError flattens a validator error chain into one English sentence
// naming the offending field, so the client gets an actionable message.
func firstFieldError(err error) string {
	var ve val.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	if len(ve) == 0 {
		return "request failed validation"
	}
	field := ve[0].Field()
	return fmt.Sprintf("field %s failed validation: %s", field, translateTag(ve[0].Tag()))
}

// translateTag maps a validation tag to the short English phrase clients see.
func translateTag(tag string) string {
	switch tag {
	case "required":
		return "is required"
	case "min":
		return "is too short"
	case "max":
		return "is too long"
	case "oneof":
		return "has an unexpected value"
	default:
		return fmt.Sprintf("failed rule %q", tag)
	}
}
