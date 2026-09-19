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
	RTK      TokenSaverRTKResponse      `json:"rtk"`
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
// is a required pointer: a PUT replaces the document, so an absent group is a
// mistake the client hears about rather than a silent reset to defaults. The
// pointers are what make the rule real — a value struct has no way to tell an
// absent group from a group whose members are all zero.
type ReplaceTokenSaverRequest struct {
	RTK      *TokenSaverRTKRequest      `json:"rtk" validate:"required"`
	Headroom *TokenSaverHeadroomRequest `json:"headroom" validate:"required"`
	Ponytail *TokenSaverLevelRequest    `json:"ponytail" validate:"required"`
}

// TokenSaverRTKRequest is the native engine's group on the way in. The filter
// list is the allowlist SPEC-API-002 §4 defines: empty means every filter is
// eligible, and each entry must be one of the twelve names. The oneof values are
// pinned against domain.TokenSaverFilters by a test, so the tag and the engine's
// registry cannot drift apart.
type TokenSaverRTKRequest struct {
	Enabled bool     `json:"enabled"`
	Filters []string `json:"filters" validate:"omitempty,dive,oneof=git-diff git-status git-log grep find ls tree dedup-log smart-truncate read-numbered search-list build-output"`
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

// ToTokenSaverSettings converts the validated replacement body into the domain
// group the settings patch applies. A nil group is the zero group: the request
// path validates before converting, and a caller that skips validation gets the
// documented defaults rather than a panic.
func (r ReplaceTokenSaverRequest) ToTokenSaverSettings() domain.TokenSaverSettings {
	out := domain.TokenSaverSettings{RTK: domain.TokenSaverRTK{Filters: []string{}}}
	if r.RTK != nil {
		out.RTK = domain.TokenSaverRTK{Enabled: r.RTK.Enabled, Filters: filtersOrEmpty(r.RTK.Filters)}
	}
	if r.Headroom != nil {
		out.Headroom = domain.TokenSaverHeadroom{Enabled: r.Headroom.Enabled, URL: r.Headroom.URL, CompressUserMessages: r.Headroom.CompressUserMessages}
	}
	if r.Ponytail != nil {
		out.Ponytail = domain.TokenSaverToggle{Enabled: r.Ponytail.Enabled, Level: r.Ponytail.Level}
	}
	return out
}

// ToTokenSaverResponse maps the domain group onto the wire shape.
func ToTokenSaverResponse(s domain.TokenSaverSettings) TokenSaverResponse {
	return TokenSaverResponse{
		RTK:      TokenSaverRTKResponse{Enabled: s.RTK.Enabled, Filters: filtersOrEmpty(s.RTK.Filters)},
		Headroom: TokenSaverHeadroomResponse{Enabled: s.Headroom.Enabled, URL: s.Headroom.URL, CompressUserMessages: s.Headroom.CompressUserMessages},
		Ponytail: TokenSaverLevelResponse{Enabled: s.Ponytail.Enabled, Level: s.Ponytail.Level},
	}
}

// filtersOrEmpty renders an absent allowlist as `[]` rather than `null`: the
// contract's meaning of an empty list is "every filter is eligible", and a null
// member would make a panel round-trip have to guess between the two.
func filtersOrEmpty(filters []string) []string {
	if filters == nil {
		return []string{}
	}
	return filters
}
