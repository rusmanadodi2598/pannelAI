// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_model_budget.go
// @for       The budget rules the provider model test applies to one probe and to one sweep (SPEC-API-001 §7.4, draft 017 §4.10).
// @uses      internal/registry, context, time.
// @reason    A probe is only cheap because its ceiling is decided somewhere, and that decision is separable from the walk it bounds. Splitting it keeps both files inside the AGENTS.md §1.1 budget and makes the ceiling readable in one place when an operator asks why a sweep stopped.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-27
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

const (
	// ProviderModelProbeTimeout bounds one model's probe. The reference uses the
	// same ceiling (src/app/api/providers/[id]/test/testUtils.js:263) because a
	// reasoning model needs room to emit anything at all before the answer is
	// judged; a shorter budget reports a thinking model as broken.
	ProviderModelProbeTimeout = 20 * time.Second

	// ProviderModelSweepTimeout bounds one whole sweep. It sits under the
	// server's 120s WriteTimeout (cmd/app-serv/main.go:134) so a sweep answers
	// with the rows it managed to run instead of being cut off mid-response.
	ProviderModelSweepTimeout = 90 * time.Second

	// ProviderModelTestDefaultLimit is how many models a sweep runs when the
	// caller asks for none. It is small on purpose: every row is a real
	// inference call against the operator's own quota.
	ProviderModelTestDefaultLimit = 6

	// ProviderModelTestMaxLimit is the ceiling a caller may ask for. The
	// reference fans a provider's whole catalog out at once
	// (test-models/route.js:51, no cap); this sweep is sequential and bounded,
	// because one click must not spend a hundred completions.
	ProviderModelTestMaxLimit = 20

	// ProviderModelTimeoutCode names a probe that ran out of its own budget. It
	// is not an internal failure: the model may answer, just not inside the
	// seconds the probe was allowed.
	ProviderModelTimeoutCode = "MODEL_TEST_TIMEOUT"

	// ProviderModelStoppedDeadline marks a sweep that ran out of budget before
	// it reached every model, so Tested reads as less than Total for a reason.
	ProviderModelStoppedDeadline = "deadline"

	// providerModelTimeoutMessage is the English line a timed-out row carries.
	providerModelTimeoutMessage = "the model did not answer within the probe budget"
)

// probeBudget clamps one probe to the budget the sweep has left, so the last
// model of a sweep is not given a fresh full ceiling on top of the response
// deadline the server is already counting down.
func probeBudget(leftover, ceiling time.Duration) time.Duration {
	if leftover >= ceiling {
		return ceiling
	}
	// No floor: a non-positive leftover means the sweep is already past its end, and
	// an expired budget answers as the timeout it is rather than a fresh twenty seconds.
	return leftover
}

// sweepEnd is the instant this sweep must be finished by: the caller's own
// deadline when it is sooner than the sweep budget, and the sweep budget
// otherwise.
func sweepEnd(ctx context.Context) time.Time {
	end := time.Now().Add(ProviderModelSweepTimeout)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(end) {
		return deadline
	}
	return end
}

// providerModelTestBudget turns the caller's request into a number of probes,
// clamped to the ceiling so one click has a stated worst-case cost.
func providerModelTestBudget(limit int) int {
	switch {
	case limit <= 0:
		return ProviderModelTestDefaultLimit
	case limit > ProviderModelTestMaxLimit:
		return ProviderModelTestMaxLimit
	default:
		return limit
	}
}

// chatModels keeps the models a chat probe can reach, in catalog order.
func chatModels(models []registry.Model) []registry.Model {
	out := make([]registry.Model, 0, len(models))
	for _, model := range models {
		if model.IsChat() {
			out = append(out, model)
		}
	}
	return out
}

// modelByID finds a model by either identifier: the id the panel shows, or the
// id the upstream expects, since a custom node's rows can differ from both.
func modelByID(models []registry.Model, id string) (registry.Model, bool) {
	for _, model := range models {
		if model.ID == id || model.UpstreamID() == id {
			return model, true
		}
	}
	return registry.Model{}, false
}
