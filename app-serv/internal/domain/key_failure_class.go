// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/key_failure_class.go
// @for       The parking classes an upstream failure is sorted into, which of them implicate a credential, and the token each is stored under.
// @uses      internal/domain/upstream_key_health.go for the windows each class owns.
// @reason    The class is the one value both halves of the health model read: the ladder picks its window by it and the store picks its counter column by its token. Naming it apart from the windows keeps a rename of a stored token from being mistaken for a change of a duration.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

// KeyFailureClass is the class an upstream failure belongs to. The class decides
// how long the key is parked (SPEC-API-001 §7.5), following the reference's
// classification: a credential-shaped failure parks long, a rate limit backs
// off exponentially, a transient outage parks briefly, and a failure the
// credential did not cause parks nothing.
type KeyFailureClass int

const (
	// KeyFailureAuth covers 401/403: the credential itself was refused.
	KeyFailureAuth KeyFailureClass = iota
	// KeyFailureRateLimit covers 429.
	KeyFailureRateLimit
	// KeyFailureTransient covers an upstream 5xx: the target is unhealthy.
	KeyFailureTransient
	// KeyFailureRequest covers every other 4xx: the request is the cause.
	KeyFailureRequest
	// KeyFailureTarget covers an upstream 404 and a call that never reached the
	// upstream: the target or the network caused it, and another credential on
	// the same body could still succeed.
	KeyFailureTarget
	// KeyFailureBilling covers an upstream 402: the credential is valid and the
	// account behind it has no credit, a condition measured in hours rather than
	// in minutes, and one the operator fixes by topping the account up.
	KeyFailureBilling
	// KeyFailureQuota covers an allowance spent for the period: the credential is
	// valid and the account behind it can serve again, but not until the vendor's
	// own reset, which is longer than any window the other ladders reach.
	KeyFailureQuota
	// KeyFailureGate covers an upstream that refuses this gateway rather than this
	// credential. Nothing about the key can change it, so it parks nothing.
	KeyFailureGate
)

// ImplicatesCredential reports whether a failure of this class says anything
// about the credential that served it. A new class opts in here rather than by
// being absent from a blacklist, because the default for a class nobody thought
// about is to leave the credential alone. A bill does implicate it: the account
// behind that key cannot serve, so the key leaves rotation on its own ladder.
func (c KeyFailureClass) ImplicatesCredential() bool {
	return c == KeyFailureAuth || c == KeyFailureRateLimit || c == KeyFailureTransient ||
		c == KeyFailureBilling || c == KeyFailureQuota
}

// String names the class for a store that has to route one increment by it, and
// for a log line. The values are stable: the key-health statement compares
// against them, so a rename here is a migration, not a cosmetic edit.
func (c KeyFailureClass) String() string {
	switch c {
	case KeyFailureAuth:
		return "auth"
	case KeyFailureRateLimit:
		return "ratelimit"
	case KeyFailureTransient:
		return "transient"
	case KeyFailureRequest:
		return "request"
	case KeyFailureTarget:
		return "target"
	case KeyFailureBilling:
		return "billing"
	case KeyFailureQuota:
		return "quota"
	case KeyFailureGate:
		return "gate"
	default:
		return "unknown"
	}
}
