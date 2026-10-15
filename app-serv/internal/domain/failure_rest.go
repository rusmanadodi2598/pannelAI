// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/failure_rest.go
// @for       What one classified upstream failure hands the health model: the rest it earned, where that number came from, and the action the router takes with it.
// @uses      time, internal/domain/failure_class.go and failure_class_policy.go for the class and windows it carries.
// @reason    A park needs to say more than "wait": whether the vendor promised the instant, merely named a header, or said nothing and the gateway guessed. That difference is what an operator reads on a parked key, and it is what the health model resolves its window from.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-15
package domain

import "time"

// ResetSource says where a rest came from, so an operator reading a parked key
// can tell a warranty from a guess.
type ResetSource string

const (
	// ResetSourceHeader is a Retry-After header the vendor sent.
	ResetSourceHeader ResetSource = "header"
	// ResetSourceBody is a reset time the vendor stated in its own body text.
	ResetSourceBody ResetSource = "body"
	// ResetSourceDefault is a wait the gateway chose, from the class ladder.
	ResetSourceDefault ResetSource = "default"
)

// RestDirective is what one classified upstream failure tells the health model:
// how long to stay away and why. The zero value is today's behaviour, a park
// computed from the class ladder alone, which is what every caller that has no
// classification to hand still gets.
type RestDirective struct {
	// Class is the recognised failure, or empty for an unclassified one.
	Class FailureClass
	// RetryAfter is the wait the vendor named on the response, or zero.
	RetryAfter time.Duration
	// ResetAt is the absolute instant the vendor said the key may serve again,
	// or zero when it named none. It is parsed out of the body, not inferred.
	ResetAt time.Time
	// Source says which of the three produced the rest.
	Source ResetSource
	// SignatureID names the signature that recognised the failure, for the log
	// and for the panel's own diagnosis.
	SignatureID string
	// Policy is the signature's own action and windows, when the signature that
	// recognised the failure carries one. It sits under the provider's override:
	// a pattern in code is a guess about a vendor, a setting is what the operator
	// decided about it.
	Policy FailureClassPolicy
}

// Label is the token recorded in last_error_class for this failure: the
// recognised class when there is one, the parking class otherwise.
func (d RestDirective) Label(class KeyFailureClass) string {
	if d.Class == "" {
		return class.String()
	}
	return d.Class.Label()
}

// SourceLabel answers the source of the rest for a store that must not hold an
// empty string: a park with no vendor statement to point at is a default.
func (d RestDirective) SourceLabel() string {
	if d.Source == "" {
		return string(ResetSourceDefault)
	}
	return string(d.Source)
}

// ActionFor answers what the router does with this failure where no parking
// budget is read: the signature's own answer when it gives one, the parking
// class's otherwise. A class that parks nothing has no window to look up, so this
// is the whole of its configuration.
func (d RestDirective) ActionFor(class KeyFailureClass) FailureAction {
	return d.Policy.ActionOrDefault(defaultActionFor(class))
}
