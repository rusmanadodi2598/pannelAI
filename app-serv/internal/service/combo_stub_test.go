// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/combo_stub_test.go
// @for       The in-memory combo repository and rotation store for the service tests.
// @uses      internal/domain, internal/repository, internal/registry.
// @reason    The catalog stub and the combo stubs are separate declarations; AGENTS.md §1.1 caps a file at 250 lines, so the latter moved here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package service

import (
	"context"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

func (r *stubComboRepo) Delete(_ context.Context, id string) error {
	combo, ok := r.byID[id]
	if !ok {
		return domain.ErrComboNotFound
	}
	delete(r.nameID, combo.Name())
	delete(r.byID, id)
	return nil
}

// stubRotation is an in-memory ComboRotationStore. It advances the same state
// the pure rotation does, so a service test proves the wiring without Redis.
type stubRotation struct {
	state   domain.RotationState
	calls   int
	resets  int
	failure error
}

func (s *stubRotation) Next(_ context.Context, _ string, models []string, stickyLimit int) ([]string, error) {
	if s.failure != nil {
		return nil, s.failure
	}
	s.calls++
	order, next := domain.ComboRoundRobin.NextOrder(models, stickyLimit, s.state)
	s.state = next
	return order, nil
}

// Reset clears the rotation state, which is what the service calls when an edit
// invalidates the stored index. It counts the resets so a test can tell a real
// reset from a silent no-op.
func (s *stubRotation) Reset(_ context.Context, _ string) error {
	s.resets++
	s.state = domain.RotationState{}
	return nil
}
