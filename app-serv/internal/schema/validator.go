// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/validator.go
// @for       The single go-playground/validator engine shared by every request.
// @uses      go-playground/validator/v10.
// @reason    AGENTS.md "Stack" names struct-tag validation as one of the two
//
//	unavoidable third-party dependencies; constructing it once avoids
//	re-compiling the rules on every request (validator is not safe to
//	share while being mutated, so it is read-only after init).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-16
package schema

import (
	"encoding/json"
	"io"
	"net/url"
	"reflect"
	"strings"
	"sync"

	val "github.com/go-playground/validator/v10"
)

var (
	engOnce sync.Once
	eng     *val.Validate
)

// engine returns the process-wide validator, initialized exactly once.
func engine() *val.Validate {
	engOnce.Do(func() {
		v := val.New()
		v.SetTagName("validate")
		registerClearingURLRules(v)
		eng = v
	})
	return eng
}

// registerClearingURLRules adds the two URL rules the §7.14 PATCH needs, where
// an empty string clears a stored value rather than being a malformed URL.
//
// The built-in `url` and `http_url` reject "", and a pointer field the panel
// sends to clear one would be refused with a validation error the operator has
// no way around: the field is only writable through this PATCH. The rules
// below repeat the built-in checks for a non-empty value and let "" through.
func registerClearingURLRules(v *val.Validate) {
	for _, rule := range []struct {
		tag      string
		httpOnly bool
	}{
		{tag: "clearing_url"},
		{tag: "clearing_http_url", httpOnly: true},
	} {
		httpOnly := rule.httpOnly
		if err := v.RegisterValidation(rule.tag, func(fl val.FieldLevel) bool {
			return clearingURL(fl, httpOnly)
		}); err != nil {
			panic("schema: registering the clearing URL rule " + rule.tag + ": " + err.Error())
		}
	}
}

// clearingURL reports whether a URL field is either empty (the clear) or a URL
// the dialer can use. The check mirrors the validator's own `url` and
// `http_url` so a value that was accepted before this rule still is.
func clearingURL(fl val.FieldLevel, httpOnly bool) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		return false
	}
	raw := strings.ToLower(field.String())
	if raw == "" {
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return false
	}
	if httpOnly {
		return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
	}
	if parsed.Scheme == "file" {
		return parsed.Path != "" && parsed.Path != "/"
	}
	return parsed.Host != "" || parsed.Fragment != "" || parsed.Opaque != ""
}

// jsonDecoder wraps encoding/json with the settings this package relies on:
// unknown fields are an error, so a misspelled key is surfaced instead of
// silently ignored (AGENTS.md §2.4 contract discipline).
func jsonDecoder(r io.Reader) *json.Decoder {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	return d
}
