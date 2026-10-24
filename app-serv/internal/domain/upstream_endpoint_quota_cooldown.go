// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_quota_cooldown.go
// @for       The endpoint's quota-poller cooldown: what a vendor 429 in real traffic says about the account's next usage read.
// @uses      time.
// @reason    SPEC-API-001 §7.12 has the poller ask a provider about each account on a floor, and a vendor already throttling an account answers a usage read with another 429. Draft 050 T1 found the column that was supposed to carry that signal was never written; this is the state the write produces, kept off Available() on purpose so a throttle on one key cannot withhold a whole account from serving.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-13
package domain

import "time"

// QuotaCooldownWindow is how long the published-quota poller stays off an account
// after the vendor throttled it in real traffic. It is longer than the slowest
// floor the poller sets for itself (claude, 10m), so the cooldown moves the next
// read for every family instead of hiding inside a floor already enforced, and half
// its failure ceiling (30m), so one 429 cannot take an account's quota screen
// stale for longer than a sustained failure run would.
const QuotaCooldownWindow = 15 * time.Minute

// QuotaCooldownUntil is when the poller may read this account again, or nil when
// nothing is holding it off.
func (e UpstreamEndpoint) QuotaCooldownUntil() *time.Time { return e.quotaCooldownUntil }

// QuotaThrottled reports whether a usage read of this endpoint would land inside
// a throttle the vendor already showed it. The deadline expires on its own: nothing
// sweeps it, which is the same rule the key circuit parks by.
func (e UpstreamEndpoint) QuotaThrottled(now time.Time) bool {
	return e.quotaCooldownUntil != nil && now.Before(*e.quotaCooldownUntil)
}

// MarkQuotaThrottled records that the vendor throttled this account, and pushes the
// poller's next read out by the cooldown window.
//
// The later deadline wins, so a second 429 extends the wait and an earlier one
// cannot shorten it: the streak of throttles is the evidence, and the account earns
// the poller back by serving traffic that is not refused. Nothing here touches the
// endpoint's status or its serving availability (§7.5), which is the whole reason
// this is a separate window from the key's park.
func (e *UpstreamEndpoint) MarkQuotaThrottled(now time.Time) {
	until := now.Add(QuotaCooldownWindow)
	if e.quotaCooldownUntil != nil && e.quotaCooldownUntil.After(until) {
		return
	}
	e.quotaCooldownUntil = &until
	e.updatedAt = now
}
