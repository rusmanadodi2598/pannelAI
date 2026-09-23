// Command app-serv shapes the requests a credential check sends.
//
// @file      cmd/app-serv/provider_validate_target_test.go
// @for       The validate-URL derivation: a declared URL wins, and a chat-path
//
//	base yields one.
//
// @uses      internal/registry, testing.
// @reason    Draft 017 §4.2's second measured consequence is that 18 of 94
//
//	providers declared a validation endpoint while 19 more name a base
//	that yields one by the reference's own rule. That rule is a small
//	pure function, so it is asserted as a table rather than through a
//	request.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestValidateTarget_DerivesFromTheBaseURL is the second consequence draft §4.2
// measured: 76 providers declared no validate URL, and 19 of them name a chat
// path that yields one.
func TestValidateTarget_DerivesFromTheBaseURL(t *testing.T) {
	cases := []struct {
		name  string
		entry registry.Provider
		want  string
	}{
		{
			name: "a declared validate URL wins",
			entry: registry.Provider{Transport: registry.Transport{
				BaseURL: "https://x.test/v1/chat/completions", ValidateURL: "https://x.test/v1/models"}},
			want: "https://x.test/v1/models",
		},
		{
			name:  "a chat completions base derives the models path",
			entry: registry.Provider{Transport: registry.Transport{BaseURL: "https://api.openai.com/v1/chat/completions"}},
			want:  "https://api.openai.com/v1/models",
		},
		{
			name:  "a chatbot base derives the models path",
			entry: registry.Provider{Transport: registry.Transport{BaseURL: "https://x.test/chatbot"}},
			want:  "https://x.test/models",
		},
		{
			name:  "a base naming neither returns nothing",
			entry: registry.Provider{Transport: registry.Transport{BaseURL: "https://api.anthropic.com/v1/messages"}},
			want:  "",
		},
		{
			name:  "an empty base returns nothing",
			entry: registry.Provider{Transport: registry.Transport{}},
			want:  "",
		},
		{
			name:  "whitespace is trimmed",
			entry: registry.Provider{Transport: registry.Transport{ValidateURL: "  https://x.test/v1/models  "}},
			want:  "https://x.test/v1/models",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateTarget(tc.entry); got != tc.want {
				t.Fatalf("validateTarget() = %q, want %q", got, tc.want)
			}
		})
	}
}

// contains is a substring test, kept local so this file needs no strings import
// for one call.
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
