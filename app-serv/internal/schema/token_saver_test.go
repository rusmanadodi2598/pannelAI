// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/token_saver_test.go
// @for       Table-driven validation of the §7.9 replacement body.
// @uses      internal/domain, reflect, strings, testing.
// @reason    AGENTS.md §2.4 requires the contract validated at the boundary,
//
//	and TDD.md §2.5 requires the table to cover the whole input
//	surface, not one happy sample: every group, every level word, the
//	filter vocabulary, and the URL schemes the saver cannot dial. The
//	tag-versus-registry test is what keeps the oneof list and
//	domain.TokenSaverFilters from drifting apart.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestReplaceTokenSaverRequest_Validation drives the §7.9 body through the same
// decode and validate calls the handler makes. The allowed cases are every
// level word, the empty and the full filter list, and both URL states, so a
// valid configuration is never refused for the wrong reason.
func TestReplaceTokenSaverRequest_Validation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "every group present",
			body:    `{"rtk":{"enabled":true,"filters":["git-diff","grep"]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: false,
		},
		{
			name:    "an empty filter list means every filter is eligible",
			body:    `{"rtk":{"enabled":true,"filters":[]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":true,"level":"ultra"}}`,
			wantErr: false,
		},
		{
			name:    "filters without the enabled member",
			body:    `{"rtk":{"filters":["ls","tree"]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"lite"}}`,
			wantErr: false,
		},
		{
			name:    "headroom with an http url while disabled",
			body:    `{"rtk":{"enabled":false,"filters":[]},"headroom":{"enabled":false,"url":"http://localhost:8787","compress_user_messages":true},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: false,
		},
		{
			name:    "headroom with an https url",
			body:    `{"rtk":{"enabled":false,"filters":[]},"headroom":{"enabled":true,"url":"https://compress.internal:8787","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: false,
		},
		{
			name:    "a missing rtk group",
			body:    `{"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a missing headroom group",
			body:    `{"rtk":{"enabled":true,"filters":[]},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a null rtk group",
			body:    `{"rtk":null,"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a missing ponytail group",
			body:    `{"rtk":{"enabled":true,"filters":[]},"headroom":{"enabled":false,"url":"","compress_user_messages":false}}`,
			wantErr: true,
		},
		{
			name:    "an empty body",
			body:    `{}`,
			wantErr: true,
		},
		{
			name:    "an unknown level word",
			body:    `{"rtk":{"enabled":true,"filters":[]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"maximum"}}`,
			wantErr: true,
		},
		{
			name:    "an empty level",
			body:    `{"rtk":{"enabled":true,"filters":[]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":""}}`,
			wantErr: true,
		},
		{
			name:    "a filter the engine does not implement",
			body:    `{"rtk":{"enabled":true,"filters":["summarize"]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a filter alias instead of the canonical name",
			body:    `{"rtk":{"enabled":true,"filters":["rg"]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "the rtk level the contract removed",
			body:    `{"rtk":{"enabled":true,"level":"full","filters":[]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a headroom url with a non-http scheme",
			body:    `{"rtk":{"enabled":false,"filters":[]},"headroom":{"enabled":true,"url":"ftp://localhost:8787","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "a headroom url that is not a URL",
			body:    `{"rtk":{"enabled":false,"filters":[]},"headroom":{"enabled":true,"url":"nearby machine","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"}}`,
			wantErr: true,
		},
		{
			name:    "an unknown field",
			body:    `{"rtk":{"enabled":true,"filters":[]},"headroom":{"enabled":false,"url":"","compress_user_messages":false},"ponytail":{"enabled":false,"level":"full"},"caveman":{"enabled":true}}`,
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

// TestTokenSaverFilterTags_MatchTheEngineVocabulary pins the oneof tags against
// domain.TokenSaverFilters. The tag is what rejects a bad name at the boundary
// and the slice is what the engine resolves, so a name added to one without the
// other is a silent hole: the request would be accepted and then ignored.
func TestTokenSaverFilterTags_MatchTheEngineVocabulary(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{"the §7.9 PUT", reflect.TypeOf(TokenSaverRTKRequest{})},
		{"the §7.14 PATCH", reflect.TypeOf(TokenSaverRTKPatch{})},
	}
	for _, tc := range cases {
		field, ok := tc.typ.FieldByName("Filters")
		if !ok {
			t.Fatalf("%s carries no Filters field", tc.name)
		}
		got := oneofValues(field.Tag.Get("validate"))
		if !reflect.DeepEqual(got, domain.TokenSaverFilters) {
			t.Fatalf("%s oneof list = %v, want the domain vocabulary %v", tc.name, got, domain.TokenSaverFilters)
		}
	}
}

// oneofValues extracts the space-separated values of a oneof tag.
func oneofValues(tag string) []string {
	for _, part := range strings.Split(tag, ",") {
		if after, ok := strings.CutPrefix(part, "oneof="); ok {
			return strings.Fields(after)
		}
	}
	return nil
}

// TestReplaceTokenSaverRequest_ToDomain pins the wire-to-domain mapping: the
// converter carries every field, so a filter or flag cannot be lost between the
// boundary and the settings patch that stores it.
func TestReplaceTokenSaverRequest_ToDomain(t *testing.T) {
	req := ReplaceTokenSaverRequest{
		RTK:      &TokenSaverRTKRequest{Enabled: true, Filters: []string{"git-diff", "build-output"}},
		Headroom: &TokenSaverHeadroomRequest{Enabled: true, URL: "https://compress.internal:8787", CompressUserMessages: true},
		Ponytail: &TokenSaverLevelRequest{Enabled: true, Level: "lite"},
	}
	got := req.ToTokenSaverSettings()
	want := domain.TokenSaverSettings{
		RTK:      domain.TokenSaverRTK{Enabled: true, Filters: []string{"git-diff", "build-output"}},
		Headroom: domain.TokenSaverHeadroom{Enabled: true, URL: "https://compress.internal:8787", CompressUserMessages: true},
		Ponytail: domain.TokenSaverToggle{Enabled: true, Level: "lite"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToTokenSaverSettings() = %+v, want %+v", got, want)
	}
}

// TestToTokenSaverResponse pins the response shape: a level is always emitted,
// and an absent allowlist renders as an empty array rather than null, so a panel
// that PUTs back what it GET never loses a field or guesses a meaning.
func TestToTokenSaverResponse(t *testing.T) {
	body := ToTokenSaverResponse(domain.TokenSaverSettings{
		RTK:      domain.TokenSaverRTK{Enabled: false},
		Headroom: domain.TokenSaverHeadroom{Enabled: false, URL: "", CompressUserMessages: false},
		Ponytail: domain.TokenSaverToggle{Enabled: false, Level: "full"},
	})
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	for _, want := range []string{`"filters":[]`, `"level":"full"`, `"rtk"`, `"headroom"`, `"ponytail"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("response %s must contain %s", encoded, want)
		}
	}
	if strings.Contains(string(encoded), `"level":""`) {
		t.Fatalf("response %s must never render an empty level", encoded)
	}
	if strings.Contains(string(encoded), `"filters":null`) {
		t.Fatalf("response %s must render the allowlist as an array", encoded)
	}
}
