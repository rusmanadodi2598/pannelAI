// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/systemone.go
// @for       The System One (Jev) decision request and answer contracts.
// @uses      bytes, encoding/json, internal/domain.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/systemone for models
//
//	declaring `kind: "systemone"`. The payload is the provider's own
//	vocabulary rather than a chat request, and the reference forwards it
//	untouched, so the fields the gateway must know are exactly the three
//	that decide whether the call is well formed: the model, the state, and
//	the questions map. Everything inside a question travels verbatim,
//	because question-level shape is the upstream's business
//	(open-sse/handlers/systemoneCore.js:28-36).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-24
package schema

import (
	"bytes"
	"encoding/json"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SystemOneRequest is the body of POST /api/v1/systemone.
//
// State is the situation to evaluate and Questions is the map of decisions to
// ask about it, keyed by the caller's own name for each question. Both are
// required: the reference refuses a body without either, and a gateway that
// forwarded one would spend an upstream call on a request that cannot be
// answered.
type SystemOneRequest struct {
	Model     string                     `json:"model" validate:"required,min=1,max=200"`
	State     json.RawMessage            `json:"state"`
	Questions map[string]json.RawMessage `json:"questions"`
}

// DecodeSystemOneRequest reads the request body into the typed contract.
//
// The two free-form members are decoded as raw JSON on purpose: their inner
// shape belongs to the upstream, so a typed struct would either reject a valid
// request or silently drop what it does not model. What the gateway does check
// is their presence and their outer type, which is the boundary the reference
// checks too.
func DecodeSystemOneRequest(raw []byte) (SystemOneRequest, error) {
	var req SystemOneRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		// A body carrying a member the contract does not declare is refused
		// rather than forwarded: the reference's own core spreads the body into
		// its payload, so an unknown member would reach the upstream and change
		// the request the client thought it made.
		return SystemOneRequest{}, domain.NewValidationError("the request body is not a valid System One request")
	}
	return req, nil
}

// Validate applies the rules the reference enforces before it calls upstream
// (open-sse/handlers/systemoneCore.js:30-36): a state that is present and not
// null, and a questions map that is a non-empty object rather than an array.
func (r SystemOneRequest) Validate() error {
	if err := ValidateStruct(r); err != nil {
		return err
	}
	if len(bytes.TrimSpace(r.State)) == 0 || string(bytes.TrimSpace(r.State)) == "null" {
		return domain.NewValidationError("field State is required")
	}
	if len(r.Questions) == 0 {
		return domain.NewValidationError("field Questions is required and must not be empty")
	}
	for name, question := range r.Questions {
		if len(bytes.TrimSpace(question)) == 0 || string(bytes.TrimSpace(question)) == "null" {
			return domain.NewValidationError("question " + name + " must not be empty")
		}
	}
	return nil
}

// UpstreamBody builds the payload the decision endpoint receives: the client's
// own body with the resolved upstream model id in place, which is what the
// reference forwards (systemoneCore.js:53, `{...body, model}`).
func (r SystemOneRequest) UpstreamBody(upstreamModel string) ([]byte, error) {
	body := map[string]json.RawMessage{
		"model":     mustSystemOneJSON(upstreamModel),
		"state":     r.State,
		"questions": mustSystemOneJSONMap(r.Questions),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, domain.NewValidationError("the request could not be encoded for the upstream")
	}
	return encoded, nil
}

// SystemOneAnswer is the decision endpoint's own answer, forwarded as it arrived.
//
// The reference reads only `usage` from it and returns the rest untouched
// (systemoneCore.js:80-93), because the answer's shape is the provider's: a
// decision model reports per-question confidences rather than a chat message, and
// reshaping it here would invent a vocabulary the client did not ask for.
type SystemOneAnswer struct {
	// Model is the model that answered, as the upstream reported it.
	Model string `json:"model,omitempty"`
	// Answers carries one entry per question the caller asked.
	Answers map[string]json.RawMessage `json:"answers,omitempty"`
	// Usage is the token accounting the upstream reported, when it did.
	Usage *SystemOneUsage `json:"usage,omitempty"`
	// Raw is the upstream's answer verbatim, which is what a caller receives.
	Raw json.RawMessage `json:"-"`
}

// SystemOneUsage is the accounting block a decision answer reports.
type SystemOneUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// mustSystemOneJSON encodes one value the connector built itself.
func mustSystemOneJSON(value string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

// mustSystemOneJSONMap encodes a map of already encoded values.
func mustSystemOneJSONMap(values map[string]json.RawMessage) json.RawMessage {
	encoded, err := json.Marshal(values)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
