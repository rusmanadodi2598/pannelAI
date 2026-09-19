// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_record_stub_test.go
// @for       The accounting doubles and fixture the media recording tests drive.
// @uses      internal/dataplane, internal/domain, context, testing.
// @reason    The recording tests assert field by field, so the recorder doubles
//
//	collect what they were handed rather than rehydrating an aggregate.
//	Keeping them beside the fixture leaves the test file itself about the
//	rules under test, and keeps both files inside the §1.1 budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// stubUsageRecorder collects the usage rows the data plane records.
type stubUsageRecorder struct {
	rows   []domain.UsageRecordInput
	failed error
}

func (r *stubUsageRecorder) Record(_ context.Context, in domain.UsageRecordInput) (domain.UsageRecord, error) {
	r.rows = append(r.rows, in)
	return domain.UsageRecord{}, r.failed
}

// stubLogRecorder collects the request logs the data plane records.
type stubLogRecorder struct {
	rows   []domain.RequestLogInput
	failed error
}

func (r *stubLogRecorder) Record(_ context.Context, in domain.RequestLogInput) (domain.RequestLog, error) {
	r.rows = append(r.rows, in)
	return domain.RequestLog{}, r.failed
}

// recordingMediaFixture builds the media service over the standard doubles with
// both recorders wired and a fixed request id, so one call's rows can be read
// field by field.
func recordingMediaFixture(t *testing.T) (*MediaCallService, *stubMediaCaller, *stubUsageRecorder, *stubLogRecorder) {
	t.Helper()
	svc, caller, _ := mediaCallFixture(t)
	usage, logs := &stubUsageRecorder{}, &stubLogRecorder{}
	svc.recorder = newDataPlaneRecorder(usage, logs, func(context.Context) string { return "req_recorded" })
	return svc, caller, usage, logs
}
