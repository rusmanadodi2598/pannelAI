// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/token_saver.go
// @for       The §7.9 token-saver contract: the standalone configuration
//
//	document and its replacement body.
//
// @uses      internal/domain (TokenSaverSettings and its groups).
// @reason    SPEC-API-001 §7.9 serves the saver configuration as its own
//
//	endpoint so the panel edits it without reading the whole §7.14
//	document, and the write is a whole replacement. One shape for the
//	read and the write keeps the GET-what-you-PUT round-trip exact.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TokenSaverResponse is GET and PUT /api/v1/token-saver (§7.9).
//
// It renders the same groups the §7.14 settings document renders, but a saver
// group's level is always emitted: the dedicated endpoint is a configuration
// file the panel round-trips, and a silently omitted level would come back as a
// missing field on the next PUT.
type TokenSaverResponse struct {
	RTK      TokenSaverLevelResponse    `json:"rtk"`
	Headroom TokenSaverHeadroomResponse `json:"headroom"`
	Ponytail TokenSaverLevelResponse    `json:"ponytail"`
}

// TokenSaverLevelResponse is one enable/level saver group with its level
// always present.
type TokenSaverLevelResponse struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

// ReplaceTokenSaverRequest is the body of PUT /api/v1/token-saver. Every group
// is required: a PUT replaces the document, so an absent group is a mistake the
// client hears about rather than a silent reset to defaults.
type ReplaceTokenSaverRequest struct {
	RTK      TokenSaverLevelRequest    `json:"rtk" validate:"required"`
	Headroom TokenSaverHeadroomRequest `json:"headroom" validate:"required"`
	Ponytail TokenSaverLevelRequest    `json:"ponytail" validate:"required"`
}

// TokenSaverLevelRequest is one enable/level saver group on the way in. The
// level set is the one the §7.14 PATCH accepts, so both write paths reject the
// same values.
type TokenSaverLevelRequest struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level" validate:"required,oneof=lite full ultra"`
}

// TokenSaverHeadroomRequest is the external compression group on the way in.
// The URL is optional because the group is usually disabled, but a value that
// is present must be an absolute http(s) URL: the saver dials it later, and a
// scheme a dial cannot speak must be refused at write time.
type TokenSaverHeadroomRequest struct {
	Enabled              bool   `json:"enabled"`
	URL                  string `json:"url" validate:"omitempty,http_url,max=2048"`
	CompressUserMessages bool   `json:"compress_user_messages"`
}

// ToTokenSaverSettings converts the replacement body into the domain group the
// settings patch applies.
func (r ReplaceTokenSaverRequest) ToTokenSaverSettings() domain.TokenSaverSettings {
	return domain.TokenSaverSettings{
		RTK:      domain.TokenSaverToggle{Enabled: r.RTK.Enabled, Level: r.RTK.Level},
		Headroom: domain.TokenSaverHeadroom{Enabled: r.Headroom.Enabled, URL: r.Headroom.URL, CompressUserMessages: r.Headroom.CompressUserMessages},
		Ponytail: domain.TokenSaverToggle{Enabled: r.Ponytail.Enabled, Level: r.Ponytail.Level},
	}
}

// ToTokenSaverResponse maps the domain group onto the wire shape.
func ToTokenSaverResponse(s domain.TokenSaverSettings) TokenSaverResponse {
	return TokenSaverResponse{
		RTK:      TokenSaverLevelResponse{Enabled: s.RTK.Enabled, Level: s.RTK.Level},
		Headroom: TokenSaverHeadroomResponse{Enabled: s.Headroom.Enabled, URL: s.Headroom.URL, CompressUserMessages: s.Headroom.CompressUserMessages},
		Ponytail: TokenSaverLevelResponse{Enabled: s.Ponytail.Enabled, Level: s.Ponytail.Level},
	}
}
