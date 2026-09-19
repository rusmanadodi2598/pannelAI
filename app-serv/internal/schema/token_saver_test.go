// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/token_saver_test.go
// @for       Table-driven validation of the §7.9 replacement body.
// @uses      internal/domain, strings, testing.
// @reason    AGENTS.md §2.4 requires the contract validated at the boundary,
//
//	and TDD.md §2.5 requires the table to cover the whole input
//	surface, not one happy sample: every group, every level word, and
//	the URL schemes the saver cannot dial.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestReplaceTokenSaverRequest_Validation drives the §7.9 body through the same
// decode and validate calls the handler makes. The allowed cases are every
// level word and both URL states, so a valid configuration is never refused for
// the wrong reason.
func TestReplaceTokenSaverRequest_Validation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "every group present",
			body:    `{"rtk":{"enabled":true,"level":"full"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: false,
		},
		{
			name:    "the lite level",
			body:    `{"rtk":{"enabled":true,"level":"lite"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":true,"level":"ultra"}}`,
			wantErr: false,
		},
		{
			name:    "headroom with an http url while disabled",
			body:    `{"rtk":{"enabled":false,"level":"full"},"headroom":{"enabled":false,"url":"http://localhost:8787","compress_user_messages":true},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: false,
		},
		{
			name:    "headroom with an https url",
			body:    `{"rtk":{"enabled":false,"level":"full"},"headroom":{"enabled":true,"url":"https://compress.internal:8787","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: false,
		},
		{
			name:    "a missing rtk group",
			body:    `{"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a missing ponytail group",
			body:    `{"rtk":{"enabled":true,"level":"full"},"headroom":{"enabled":false,"url":"","compress_user_messages":false}}`,
			wantErr: true,
		},
		{
			name:    "an empty body",
			body:    `{}`,
			wantErr: true,
		},
		{
			name:    "an unknown level word",
			body:    `{"rtk":{"enabled":true,"level":"maximum"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "an empty level",
			body:    `{"rtk":{"enabled":true,"level":""},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a headroom url with a non-http scheme",
			body:    `{"rtk":{"enabled":false,"level":"full"},"headroom":{"enabled":true,"url":"ftp://localhost:8787","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a headroom url that is not a URL",
			body:    `{"rtk":{"enabled":false,"level":"full"},"headroom":{"enabled":true,"url":"nearby machine","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "an unknown field",
			body:    `{"rtk":{"enabled":true,"level":"full"},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"},"filters":["git-diff"]}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req ReplaceTokenSaverRequest
			dec := jsonDecoder(strings.NewReader(tc.body))
			err := dec.Decode(&req)
			if err == nil {
				err = ValidateStruct(req)
			}
			if tc.wantErr != (err != nil) {
				t.Fatalf("decode+validate error = %v, wantErr = %t", err, tc.wantErr)
			}
		})
	}
}

// TestReplaceTokenSaverRequest_ToDomain pins the wire-to-domain mapping: the
// converter carries every field, so a level or flag cannot be lost between the
// boundary and the settings patch that stores it.
func TestReplaceTokenSaverRequest_ToDomain(t *testing.T) {
	req := ReplaceTokenSaverRequest{
		RTK:      TokenSaverLevelRequest{Enabled: true, Level: "ultra"},
		Headroom: TokenSaverHeadroomRequest{Enabled: true, URL: "https://compress.internal:8787", CompressUserMessages: true},
		Ponytail: TokenSaverLevelRequest{Enabled: true, Level: "lite"},
	}
	got := req.ToTokenSaverSettings()
	want := domain.TokenSaverSettings{
		RTK:      domain.TokenSaverToggle{Enabled: true, Level: "ultra"},
		Headroom: domain.TokenSaverHeadroom{Enabled: true, URL: "https://compress.internal:8787", CompressUserMessages: true},
		Ponytail: domain.TokenSaverToggle{Enabled: true, Level: "lite"},
	}
	if got != want {
		t.Fatalf("ToTokenSaverSettings() = %+v, want %+v", got, want)
	}
}

// TestToTokenSaverResponse pins the response shape: a level is always emitted,
// so a panel that PUTs back what it GET never loses a field.
func TestToTokenSaverResponse(t *testing.T) {
	body := ToTokenSaverResponse(domain.TokenSaverSettings{
		RTK:      domain.TokenSaverToggle{Enabled: true, Level: "full"},
		Headroom: domain.TokenSaverHeadroom{Enabled: false, URL: "", CompressUserMessages: false},
		Ponytail: domain.TokenSaverToggle{Enabled: false, Level: "full"},
	})
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	for _, want := range []string{`"level":"full"`, `"rtk"`, `"headroom"`, `"ponytail"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("response %s must contain %s", encoded, want)
		}
	}
	if strings.Contains(string(encoded), `"level":""`) {
		t.Fatalf("response %s must never render an empty level", encoded)
	}
}
