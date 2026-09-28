// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/provider_model_probe.go
// @for       The §7.4 model-test contract: one provider's model probed by id, or
//
//	every chat model it offers probed in one bounded sweep.
//
// @uses      encoding/json tags only.
// @reason    Draft 017 §4.10 (F10) recorded that the panel could only ask "does
//
//	this connection answer" and never "does this model answer". The answer
//	is per model, so the DTO is one row per model plus the sweep's own
//	budget bookkeeping.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-27
package schema

// TestProviderModelRequest is the body of the single-model route. The model is
// named rather than addressed by a stored id because the panel tests the string
// it would put in a client.
type TestProviderModelRequest struct {
	ModelID string `json:"model_id" validate:"required,max=191"`
}

// TestProviderModelsRequest is the body of the sweep route. EOF decodes to the
// zero value, which the service reads as "use the default budget".
type TestProviderModelsRequest struct {
	Limit int `json:"limit,omitempty" validate:"omitempty,min=1"`
}

// ProviderModelTestResult is one model's probe outcome. A failed probe is a row,
// not an error: which model is down is the finding the caller asked for.
//
// Status is the upstream's own HTTP code and is carried only when the failure
// names one. A healthy probe reports latency, not a status the gateway never
// read, so the panel cannot render a 200 it did not see.
type ProviderModelTestResult struct {
	ModelID    string `json:"model_id"`
	Name       string `json:"name,omitempty"`
	OK         bool   `json:"ok"`
	LatencyMS  int64  `json:"latency_ms"`
	EndpointID string `json:"endpoint_id,omitempty"`
	Status     int    `json:"status,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ProviderModelTestResponse is the sweep's answer: the catalog it walked, how
// much of the budget it spent, and one row per model it probed.
//
// Source and Warning are the model list's own origin (SPEC-API §7.4, draft 017
// §4.9), carried because a sweep of a stale list proves less than a sweep of the
// live one. Tested and Total are the truncation an operator can act on: the rows
// that were not run are a number, not a silence.
type ProviderModelTestResponse struct {
	ProviderID string                    `json:"provider_id"`
	Source     string                    `json:"source"`
	Warning    string                    `json:"warning,omitempty"`
	Tested     int                       `json:"tested"`
	Total      int                       `json:"total"`
	Stopped    string                    `json:"stopped,omitempty"`
	Results    []ProviderModelTestResult `json:"results"`
}
