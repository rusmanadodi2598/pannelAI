// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/auth.go
// @for       Dashboard authentication request and session status contracts.
// @uses      github.com/go-playground/validator/v10 through shared validation.
// @reason    SPEC-API-001 §7.2 requires typed, validated auth payloads and a
//
//	stable status response before handlers can process credentials.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-09-17
package schema

import (
	"unicode/utf8"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// MaxPasswordBytes matches bcrypt's 72-byte input limit.
const MaxPasswordBytes = 72

// ValidatePasswordBytes rejects multi-byte strings that exceed bcrypt's byte limit.
func ValidatePasswordBytes(values ...string) error {
	for _, value := range values {
		if !utf8.ValidString(value) || len([]byte(value)) > MaxPasswordBytes {
			return domain.NewValidationError("password must be at most 72 bytes")
		}
	}
	return nil
}

// LoginRequest is the body of POST /api/v1/auth/login.
type LoginRequest struct {
	Password string `json:"password" validate:"required,min=1,max=72"`
}

// ChangePasswordRequest is the body of POST /api/v1/auth/change-password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,min=1,max=72"`
	NewPassword     string `json:"new_password" validate:"required,min=1,max=72"`
}

// AuthStatusResponse is returned by GET /api/v1/auth/status.
type AuthStatusResponse struct {
	Authenticated      bool `json:"authenticated"`
	RequireLogin       bool `json:"require_login"`
	PasswordConfigured bool `json:"password_configured"`
}
