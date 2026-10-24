// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/errors.go
// @for       The prompt error codes named by SPEC-BOT-001 §6.
// @uses      fmt.
// @reason    A refusal has to reach the user as one stable code, so the codes live in one place instead of being spelled at each call site.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import "fmt"

// Prompt error codes. BOT_PROMPT_INVALID covers a shape or field rule the
// builder should fix; the other codes name a specific refusal.
const (
	CodeInvalid     = "BOT_PROMPT_INVALID"
	CodeEmpty       = "BOT_PROMPT_EMPTY"
	CodeUnsupported = "BOT_PROMPT_UNSUPPORTED_FIELD"
	CodeRoleScope   = "BOT_PROMPT_ROLE_SCOPE"
	CodeBudget      = "BOT_PROMPT_BUDGET"
	CodePreview     = "BOT_PROMPT_PREVIEW"
)

// Error is one refused prompt with a machine-readable code.
type Error struct {
	Code    string
	Message string
}

// Error renders the code beside the field, never a user payload.
func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func invalid(message string, cause error) error {
	if cause == nil {
		return &Error{Code: CodeInvalid, Message: message}
	}
	return &Error{Code: CodeInvalid, Message: fmt.Sprintf("%s: %v", message, cause)}
}
