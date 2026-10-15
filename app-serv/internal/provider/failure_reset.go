// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/failure_reset.go
// @for       What the classifier is handed and answers, and the parsers that read a vendor's own reset out of a refusal body.
// @uses      net/http, regexp, strings, time, internal/domain.
// @reason    A refusal body states two different promises depending on the vendor: a wait in a header, or an instant in its own sentence. Reading the instant is the only thing that turns a spent allowance into one park instead of one park per request, so the shape of that answer belongs apart from the table that recognises it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"net/http"
	"regexp"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Failure is what the classifier is given: everything one refused call left
// behind. Body is the bounded failure body, never the streamed answer.
type Failure struct {
	// Provider is the registry id of the upstream that answered.
	Provider string
	// Host is the upstream's hostname. A custom node the operator created has an
	// id nobody can predict, but its base URL still names the vendor, so the two
	// together are what a signature can be scoped to.
	Host string
	// Status is the HTTP status, or zero for a failure that never got one.
	Status int
	// Header carries the vendor's own Retry-After when it sent one.
	Header http.Header
	// Body is the bounded refusal body.
	Body []byte
	// Quota reports that the connector already read a spent account out of the
	// vendor's own envelope. Only a connector that understands that envelope sets
	// it, so it is worth more than any pattern matched against the body.
	Quota bool
}

// FailureVerdict is one classified failure: what it is, when the vendor says the
// key may serve again, and where that answer came from.
type FailureVerdict struct {
	// Class is the recognised failure. It is never empty: a refusal no signature
	// recognised still answers what its status alone can carry.
	Class domain.FailureClass
	// ResetAt is the instant the vendor stated the key may serve again, or zero
	// when it stated none. It is what turns a spent allowance into one park
	// instead of one park per request.
	ResetAt time.Time
	// ResetSource says which statement produced the rest.
	ResetSource domain.ResetSource
	// RetryAfter is the wait the vendor named as a header, or zero.
	RetryAfter time.Duration
	// SignatureID names the signature that recognised the failure, for the log.
	SignatureID string
	// Policy is the signature's own action and windows, when it carries one.
	Policy domain.FailureClassPolicy

	retryable *bool
}

// Retryable reports whether the same key may be asked again inside one call: the
// signature's own answer when it gives one, otherwise the class's.
func (v FailureVerdict) Retryable() bool {
	if v.retryable != nil {
		return *v.retryable
	}
	return v.Class.Retryable()
}

// RetryableAfter sets a signature's own answer to Retryable, for a vendor whose
// status says one thing and whose body says another.
func RetryableAfter(retry bool) *bool { return &retry }

// ResetParser reads one reset instant out of a refusal body: the pattern finds
// the sentence and the layout reads the date inside it.
type ResetParser struct {
	// Pattern carries the date in its first capture group.
	Pattern *regexp.Regexp
	// Layout is the time layout of that capture, as time.Parse reads it.
	Layout string
}

// Parse reads the reset, or reports false. A date the layout cannot read is no
// reset rather than a zero time: a zero time would park the key until 1970,
// which is worse than parking it on the class ladder.
func (p *ResetParser) Parse(body []byte) (time.Time, bool) {
	if p == nil || p.Pattern == nil || p.Layout == "" {
		return time.Time{}, false
	}
	found := p.Pattern.FindSubmatch(body)
	if len(found) < 2 {
		return time.Time{}, false
	}
	when, err := time.Parse(p.Layout, string(found[1]))
	if err != nil {
		return time.Time{}, false
	}
	return when.UTC(), true
}

// genericResets are tried on any body whose signature named no reset of its own,
// so a vendor nobody has seeded yet still gets its stated date rather than the
// ladder. They are deliberately narrow: a number that happens to look like a
// date must not park a key for a week.
var genericResets = []ResetParser{
	{
		Pattern: regexp.MustCompile(`\b(\d{1,2} \w{3} \d{4} at \d{1,2}:\d{2} UTC)\b`),
		Layout:  "2 Jan 2006 at 15:04 MST",
	},
	{
		Pattern: regexp.MustCompile(`(?i)(?:resets?|renews?|refills?|starts(?: on)?|available|until)[^0-9]{0,30}(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2})?(?:Z|[+-]\d{2}:?\d{2})?)`),
		Layout:  time.RFC3339,
	},
}

// firstReset answers the first generic parser that reads a date out of the body,
// or the zero time when none of them does.
func firstReset(parsers []ResetParser, body []byte) time.Time {
	for i := range parsers {
		if when, ok := parsers[i].Parse(body); ok {
			return when
		}
	}
	return time.Time{}
}
