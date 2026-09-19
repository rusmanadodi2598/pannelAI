// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/oauth_refresh.go
// @for       The freshness rules of an OAuth token set and the endpoint
//
//	transition a dead-lettered refresh takes.
//
// @uses      internal/domain (error constructors), time.
// @reason    SPEC-API-001 §7.4 reports token expiry per endpoint and the
//
//	refresh worker (§10, P2) must decide "due" the same way the status
//	route reports it, so the derivation lives once in the domain where
//	both callers read it. MarkUnhealthy is here too because the
//	dead-letter decision is a business rule: the account stopped being
//	routable because its credential died, which is health tracking, not
//	a PATCH a client could make.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-19
package domain

import "time"

// Refresh-state names as the API reports them (SPEC-API-001 §7.4). A token
// set is exactly one of these four; "due" includes the expired case so a
// panel shows one actionable bucket instead of two.
const (
	RefreshMissing = "missing" // no expiry is known
	RefreshFresh   = "fresh"   // outside the lead window
	RefreshDue     = "due"     // inside the lead window, or already expired
)

// DefaultRefreshLead is the lead a provider that declares none still gets, so
// a worker starts refreshing before a request meets an expired token. Ten
// percent of an hour-scale token life is the smallest window that survives a
// worker tick without racing it.
const DefaultRefreshLead = 30 * time.Minute

// OAuthRefreshState derives the refresh state of a stored token set. The lead
// is the provider's refresh_lead; a non-positive lead falls back to
// DefaultRefreshLead rather than making "due" unreachable for a provider that
// never declared one.
func OAuthRefreshState(cred *OAuthCredential, lead time.Duration, now time.Time) string {
	if cred == nil || cred.ExpiresAt == nil {
		return RefreshMissing
	}
	if lead <= 0 {
		lead = DefaultRefreshLead
	}
	// The boundary itself counts as due: at exactly the lead instant,
	// refreshing is the safe side, because a later tick might miss the window.
	if !now.Add(lead).Before(*cred.ExpiresAt) {
		return RefreshDue
	}
	return RefreshFresh
}

// MarkUnhealthy moves the endpoint to the error state and records why, so the
// panel shows a reason rather than a bare status. Only health tracking calls
// this; ParseUpstreamEndpointStatus deliberately refuses "error" from the
// wire because a client must not be able to hide a usable account.
func (e *UpstreamEndpoint) MarkUnhealthy(message string, now time.Time) {
	e.status = UpstreamEndpointError
	e.RecordTest(EndpointTestFail, 0, message, now)
}
