// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_queue_failure_test.go
// @for       Tests pinning that a vendor queue recognised by its own code is reported as a throttle even when it arrived under a status that says the credential broke.
// @uses      net/http, testing, internal/domain.
// @reason    Draft 060 D1: Qoder states a queue that lifts in tens of seconds inside a body its HTTP status calls a 403. The park was fixed where the code is read; what is pinned here is the other surface, that the client's advice follows the classification rather than the status it rode in on, and that a 403 nobody recognised still answers as a refused credential.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-17
package dataplane

import (
	"net/http"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestTranslateCallError_QueueReportsAThrottle pins the client's half of the queue:
// RATE_LIMITED and a 429, so a caller's retry policy retries something it may
// actually get, and the chain keeps walking to the next credential meanwhile.
func TestTranslateCallError_QueueReportsAThrottle(t *testing.T) {
	engine := &Engine{}
	queue := &UpstreamError{
		Status: http.StatusForbidden, Message: "queue throttle",
		Class: domain.FailureRateLimitTemporary,
	}

	failure := AsError(engine.translateCallError(queue))
	if failure.Code != CodeRateLimited {
		t.Fatalf("code = %s, want %s: UPSTREAM_ERROR tells the client the gateway broke",
			failure.Code, CodeRateLimited)
	}
	if failure.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: the code decides what a client retries on", failure.Status)
	}
	if !failoverWorthy(failure.Code) {
		t.Fatal("a throttle must keep the chain walking")
	}
}

// TestTranslateCallError_Unrecognised403KeepsTheCredentialAnswer is the guard on that
// rule: the status alone is never read as a throttle, so an upstream that really
// refused a credential is not reported as a busy one.
func TestTranslateCallError_Unrecognised403KeepsTheCredentialAnswer(t *testing.T) {
	engine := &Engine{}
	refused := &UpstreamError{Status: http.StatusForbidden, Message: "invalid credentials"}

	failure := AsError(engine.translateCallError(refused))
	if failure.Code == CodeRateLimited {
		t.Fatal("a refused credential was reported as a throttle")
	}
	if failure.Status != http.StatusBadGateway {
		t.Fatalf("status = %d, want the upstream failure a credential refusal already answers", failure.Status)
	}
}
