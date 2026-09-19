// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/combo_probe.go
// @for       The combo test endpoint (SPEC-API-001 §7.7).
// @uses      internal/schema, internal/service, net/http.
// @reason    The route shares the combo resource but not its dependency: probing
//
//	needs the data plane, and the data plane is built after the combo
//	service, so the test lives in its own handler rather than widening the
//	CRUD handler's constructor. The split mirrors the endpoint family,
//	where keys and bulk onboarding already have handlers of their own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-19
package handler

import (
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// ComboTestHandler serves POST /api/v1/combos/{id}/test (§7.7).
type ComboTestHandler struct {
	tests *service.ComboTestService
}

// NewComboTestHandler validates deps and returns the handler.
func NewComboTestHandler(tests *service.ComboTestService) *ComboTestHandler {
	return &ComboTestHandler{tests: tests}
}

// Test serves the probe route. A member that failed its probe is part of the
// answer — that is what the operator asked for — so only a combo that cannot be
// read is an error; the results carry each failure's own code.
func (h *ComboTestHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, err := comboPathID(r)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	response, err := h.tests.Test(r.Context(), id)
	if err != nil {
		schema.WriteError(w, err)
		return
	}
	schema.WriteJSON(w, http.StatusOK, response)
}
