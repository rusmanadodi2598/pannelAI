// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/provider_model_probe.go
// @for       The two §7.4 model test routes: one model by id, or a bounded sweep
//
//	over a provider's chat models (draft 017 §4.10, F10).
//
// @uses      internal/schema, internal/service, net/http.
// @reason    The reference names this capability per connection
//
//	(/providers/[id]/test-models) because "the credential works" and "this
//	model answers" are different questions, and only the second one tells an
//	operator why a client call failed. Both routes answer 200 with rows: a
//	model that refused to answer is the finding, not a request failure.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability stable
// @since     2026-09-27
package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ProviderModelTestHandler serves the §7.4 model test routes.
type ProviderModelTestHandler struct {
	tests *service.ProviderModelTestService
}

// modelIDRequiredMessage names the refusal an empty body gets on the single-model
// route. The codec's own "EOF" is not a sentence an operator can act on, and the
// route's contract is that a model has to be named.
const modelIDRequiredMessage = "a model id is required"

// NewProviderModelTestHandler validates deps and returns the handler.
func NewProviderModelTestHandler(tests *service.ProviderModelTestService) *ProviderModelTestHandler {
	return &ProviderModelTestHandler{tests: tests}
}

// Model serves POST /api/v1/providers/{provider_id}/models/test.
func (h *ProviderModelTestHandler) Model(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	req, ok := decodeModelTestTarget(w, r)
	if !ok {
		return
	}
	result, err := h.tests.TestModel(r.Context(), providerID, req.ModelID)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, result)
}

// Models serves POST /api/v1/providers/{provider_id}/test-models.
func (h *ProviderModelTestHandler) Models(w http.ResponseWriter, r *http.Request) {
	providerID, ok := pathValue(w, r, "provider_id")
	if !ok {
		return
	}
	req, ok := decodeModelsTestBudget(w, r)
	if !ok {
		return
	}
	response, err := h.tests.TestModels(r.Context(), providerID, req.Limit)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, response)
}

// decodeModelTestTarget reads the single-model body, which must name a model: an
// empty body is refused rather than guessed at, because "test the first model"
// is not a question an operator asked.
func decodeModelTestTarget(w http.ResponseWriter, r *http.Request) (schema.TestProviderModelRequest, bool) {
	var req schema.TestProviderModelRequest
	if r.Body == nil {
		schema.WriteError(w, domain.NewValidationError(modelIDRequiredMessage))
		return req, false
	}
	if err := decodeInto(r, &req); err != nil {
		if errors.Is(err, io.EOF) {
			schema.WriteError(w, domain.NewValidationError(modelIDRequiredMessage))
			return req, false
		}
		schema.WriteError(w, modelTestBodyError(err))
		return req, false
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return req, false
	}
	return req, true
}

// decodeModelsTestBudget reads the sweep's body, treating an absent one as "use
// the default budget".
func decodeModelsTestBudget(w http.ResponseWriter, r *http.Request) (schema.TestProviderModelsRequest, bool) {
	var req schema.TestProviderModelsRequest
	if r.Body == nil {
		return req, true
	}
	if err := decodeInto(r, &req); err != nil {
		if errors.Is(err, io.EOF) {
			return schema.TestProviderModelsRequest{}, true
		}
		schema.WriteError(w, modelTestBodyError(err))
		return req, false
	}
	if err := schema.ValidateStruct(req); err != nil {
		schema.WriteError(w, err)
		return req, false
	}
	return req, true
}

// modelTestBodyError turns a decode failure into the structured refusal §1.3
// requires, with the parser's tail cleaned of the body it choked on.
func modelTestBodyError(err error) *domain.AppError {
	return domain.NewValidationError("invalid request body: " + cleanTail(err))
}
