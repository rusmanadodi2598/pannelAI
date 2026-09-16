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
		eng = v
	})
	return eng
}

// jsonDecoder wraps encoding/json with the settings this package relies on:
// unknown fields are an error, so a misspelled key is surfaced instead of
// silently ignored (AGENTS.md §2.4 contract discipline).
func jsonDecoder(r io.Reader) *json.Decoder {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	return d
}
