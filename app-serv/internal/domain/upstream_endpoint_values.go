// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/upstream_endpoint_values.go
// @for       The account-identity value objects an upstream endpoint carries.
// @uses      strings, time.
// @reason    The account, its email and the last connectivity test are the three values the panel and the duplicate-account rule both read, so their invariants live with them rather than in whichever caller built the struct last (AGENTS.md §2.2).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-04
package domain

import (
	"strings"
	"time"
)

// maxEmailLength is the column's practical ceiling: an address longer than this
// is not an address but a caller trying to park a payload in an identity field.
const maxEmailLength = 320

// NormalizeEmail is the one spelling an account's email is stored and compared
// with. A vendor that answers `Bob@Example.COM` today and `bob@example.com` on
// the next import would otherwise read as a second account, and the duplicate
// check that keeps one endpoint per account matches on this value.
func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Email is an account's identity handle. It guarantees the one property the
// duplicate-account check depends on, a single spelling, and it is deliberately
// not an RFC validator: a device login that answered no email carries the
// synthetic `qoder-user-<id>` here, which is an identity, not a mailbox.
type Email struct {
	value string
}

// ParseEmail normalizes an address to the form the account is stored and compared
// in. An empty value is a valid "the vendor stated no identity", not an error.
func ParseEmail(raw string) (Email, error) {
	value := NormalizeEmail(raw)
	if value == "" {
		return Email{}, nil
	}
	if strings.ContainsAny(value, " \t\n") {
		return Email{}, NewValidationError("an account identity cannot contain spaces")
	}
	if len(value) > maxEmailLength {
		return Email{}, NewValidationError("an account identity is too long to be one")
	}
	return Email{value: value}, nil
}

// String renders the address as stored. Every read of the value goes through
// this, so no caller can hand on a variant spelling.
func (e Email) String() string { return e.value }

// IsZero reports that the vendor stated no identity at all.
func (e Email) IsZero() bool { return e.value == "" }

// EndpointAccount is the non-secret identity an endpoint presents. It is what
// distinguishes two accounts of the same provider in the panel, and it is what
// makes a re-import of the same account detectable.
type EndpointAccount struct {
	name        string
	email       Email
	machineID   string
	workspaceID string
}

// EndpointAccountInput is what a caller presents for an account identity. Every
// field is non-secret; the vendor's own words, not this gateway's.
type EndpointAccountInput struct {
	Name        string
	Email       string
	MachineID   string
	WorkspaceID string
}

// NewEndpointAccount builds an account identity from what a vendor stated. The
// email is normalized on the way in, because the same account answered twice in
// two letter cases is the failure this type exists to prevent.
func NewEndpointAccount(input EndpointAccountInput) (EndpointAccount, error) {
	parsed, err := ParseEmail(input.Email)
	if err != nil {
		return EndpointAccount{}, err
	}
	return EndpointAccount{
		name:        strings.TrimSpace(input.Name),
		email:       parsed,
		machineID:   strings.TrimSpace(input.MachineID),
		workspaceID: strings.TrimSpace(input.WorkspaceID),
	}, nil
}

// RehydrateEndpointAccount rebuilds a stored identity without re-validating it.
// It is the repository load path's constructor, the counterpart of
// RehydrateUpstreamEndpoint, and a caller outside that path has no business
// bypassing the checks.
func RehydrateEndpointAccount(input EndpointAccountInput) EndpointAccount {
	return EndpointAccount{
		name:        input.Name,
		email:       Email{value: NormalizeEmail(input.Email)},
		machineID:   input.MachineID,
		workspaceID: input.WorkspaceID,
	}
}

func (a EndpointAccount) Name() string        { return a.name }
func (a EndpointAccount) Email() Email        { return a.email }
func (a EndpointAccount) MachineID() string   { return a.machineID }
func (a EndpointAccount) WorkspaceID() string { return a.workspaceID }

// HasIdentity reports that the vendor stated at least one fact about the account.
// The connect path decides on this: an answer carrying nothing gets the smallest
// `Account N` its provider does not already use, rather than a blank label.
func (a EndpointAccount) HasIdentity() bool {
	return !a.email.IsZero() || a.workspaceID != "" || a.name != ""
}

// EndpointTestState is the outcome vocabulary of a connectivity test. A test
// either answered or it did not, so there are two.
type EndpointTestState string

const (
	EndpointTestOK   EndpointTestState = "ok"
	EndpointTestFail EndpointTestState = "fail"
)

// EndpointTestStatus is the outcome of the last connectivity test.
type EndpointTestStatus struct {
	state     EndpointTestState
	latencyMS int
	checkedAt *time.Time
	message   string
}

// NewEndpointTestStatus records one test run. A state the aggregate does not name
// is not a passing test: it collapses to the failure spelling, so the stored value
// stays inside the two states the panel and the response schema can render rather
// than holding whatever a caller interpolated.
func NewEndpointTestStatus(state string, latencyMS int, message string, checkedAt time.Time) EndpointTestStatus {
	parsed := EndpointTestFail
	if strings.TrimSpace(state) == string(EndpointTestOK) {
		parsed = EndpointTestOK
	}
	checked := checkedAt.UTC()
	return EndpointTestStatus{
		state:     parsed,
		latencyMS: latencyMS,
		checkedAt: &checked,
		message:   strings.TrimSpace(message),
	}
}

// RehydrateEndpointTestStatus rebuilds a stored outcome without re-validating it,
// for the repository load path. A nil checkedAt and an empty state is the zero
// value the panel renders as "never tested".
func RehydrateEndpointTestStatus(state string, latencyMS int, message string, checkedAt *time.Time) EndpointTestStatus {
	return EndpointTestStatus{
		state:     EndpointTestState(state),
		latencyMS: latencyMS,
		checkedAt: checkedAt,
		message:   message,
	}
}

func (s EndpointTestStatus) State() EndpointTestState { return s.state }
func (s EndpointTestStatus) LatencyMS() int           { return s.latencyMS }
func (s EndpointTestStatus) Message() string          { return s.message }
func (s EndpointTestStatus) CheckedAt() *time.Time    { return s.checkedAt }

// Recorded reports whether a test ever ran. The response omits the block when it
// did not, which is why emptiness is asked here rather than guessed from a field.
func (s EndpointTestStatus) Recorded() bool {
	return s.state != "" || s.checkedAt != nil
}
