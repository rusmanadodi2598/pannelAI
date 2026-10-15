// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/failure_signature.go
// @for       The signature registry: how one upstream refusal is recognised, which class it belongs to, and when the vendor says the key may serve again.
// @uses      internal/domain, net/http, regexp, strings, sync.
// @reason    A 429 is two different failures depending on the vendor: "under heavy load" lifts in minutes, "you've used this period's free allowance" does not lift until the date the same body prints. Only the connector layer knows which vendors say which, so the patterns live here and the core asks one question.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package provider

import (
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// Signature is one recognisable upstream refusal.
type Signature struct {
	// ID names the signature in the log and in upstream_keys.signature_id.
	ID string
	// Provider scopes the signature to one registry id. Empty matches any.
	Provider string
	// Host scopes the signature to one upstream hostname, or to its subdomains.
	// Empty matches any.
	Host string
	// Status scopes the signature to one HTTP status. Zero matches any.
	Status int
	// Pattern matches the refusal body. It is the common case: a vendor's
	// throttle and its spent allowance arrive under the same status and are told
	// apart only by their wording.
	Pattern *regexp.Regexp
	// Match is the escape hatch for a refusal the connector already decoded,
	// where a regexp over the body would only re-guess what it knows.
	Match func(Failure) bool
	// Class is what the refusal is.
	Class domain.FailureClass
	// Reset reads the vendor's own reset out of the body.
	Reset *ResetParser
	// Retryable overrides the class's answer, when set.
	Retryable *bool
	// Policy overrides the class's action, ceiling and cooldown, when set.
	Policy domain.FailureClassPolicy
}

// matches reports whether this signature recognises the failure. Every scope the
// signature names must agree: an unscoped one matches on its pattern alone.
func (s Signature) matches(f Failure) bool {
	if s.Provider != "" && s.Provider != f.Provider {
		return false
	}
	if s.Host != "" && !hostMatches(f.Host, s.Host) {
		return false
	}
	if s.Status != 0 && s.Status != f.Status {
		return false
	}
	if s.Match != nil {
		return s.Match(f)
	}
	if s.Pattern == nil {
		return false
	}
	return s.Pattern.Match(f.Body)
}

// hostMatches reports whether an upstream host is the scoping host or one of its
// subdomains, so one entry covers api.tokenharbor.ai as well as tokenharbor.ai.
func hostMatches(host, scope string) bool {
	host = strings.ToLower(host)
	scope = strings.ToLower(scope)
	return host == scope || strings.HasSuffix(host, "."+scope)
}

// The seeded registry. Every entry here is a refusal measured in production:
// a class guessed from the status alone is what parks a spent allowance for five
// minutes and hands it back to the walk to be refused again.
var seedSignatures = []Signature{
	{
		// TH Harbor's throttle. The same vendor's 429 is also its spent
		// allowance, so the two are told apart by their wording and this one
		// keeps the short ceiling, honouring the Retry-After it sends.
		ID:        "tokenharbor.heavy-load",
		Host:      "tokenharbor.ai",
		Status:    http.StatusTooManyRequests,
		Pattern:   regexp.MustCompile(`(?i)under heavy load|slowed down for a few minutes`),
		Class:     domain.FailureRateLimitTemporary,
		Retryable: RetryableAfter(true),
	},
	{
		// TH Harbor's spent free allowance: the body names the reset and the key
		// must not come back before it. Measured 2026-10-15 on four of six keys,
		// each parked for the five-minute ceiling against a stated reset days
		// away, re-burning a walk leg every cycle.
		ID:      "tokenharbor.free-allowance",
		Status:  http.StatusTooManyRequests,
		Pattern: regexp.MustCompile(`(?i)used this period's free allowance`),
		Class:   domain.FailureQuotaExhausted,
		Reset: &ResetParser{
			Pattern: regexp.MustCompile(`starts on (\d{1,2} \w{3} \d{4} at \d{1,2}:\d{2} UTC)`),
			Layout:  "2 Jan 2006 at 15:04 MST",
		},
	},
	{
		// The envelope a Qoder refusal arrives in states a spent account in its
		// own code; the connector has already read it, and the core has been
		// dropping that flag. Recognising it here is what stops a spent Qoder
		// account from being classified as a refused credential.
		ID:    "qoder.envelope.quota",
		Match: func(f Failure) bool { return f.Quota },
		Class: domain.FailureQuotaExhausted,
	},
	{
		// OpenCode's free tier answers this gateway rather than this credential:
		// no key on the account can serve it and retrying only spends the walk.
		ID:        "opencode.free-tier-gate",
		Status:    http.StatusBadGateway,
		Pattern:   regexp.MustCompile(`(?i)free tier can only be used from within`),
		Class:     domain.FailureGateBlocked,
		Retryable: RetryableAfter(false),
	},
}

var (
	signatureMu sync.RWMutex
	signatures  = append([]Signature{}, seedSignatures...)
)

// RegisterSignature adds a signature at boot. It is how a vendor nobody seeded
// is taught without touching the core: the last registered signature wins, so a
// later entry can override a seeded one for the same refusal.
func RegisterSignature(signature Signature) {
	signatureMu.Lock()
	defer signatureMu.Unlock()
	signatures = append([]Signature{signature}, signatures...)
}

// ResetSignatures restores the seeded registry, for a test that registered its own.
func ResetSignatures() {
	signatureMu.Lock()
	defer signatureMu.Unlock()
	signatures = append([]Signature{}, seedSignatures...)
}

// Classify recognises one upstream refusal. A failure no signature claims still
// answers what its status alone can carry, which is the behaviour this replaced:
// the registry can only ever be more specific than the status, never less.
func Classify(f Failure) FailureVerdict {
	signatureMu.RLock()
	table := signatures
	signatureMu.RUnlock()

	for _, signature := range table {
		if !signature.matches(f) {
			continue
		}
		verdict := FailureVerdict{
			Class:       signature.Class,
			SignatureID: signature.ID,
			retryable:   signature.Retryable,
			Policy:      signature.Policy,
		}
		// The source is named only where a statement was actually found: a
		// signature that recognised the failure but read no date and no header
		// from it leaves the source empty, which stores as `default` and tells an
		// operator the wait was the gateway's own reckoning.
		if when, ok := signature.Reset.Parse(f.Body); ok {
			verdict.ResetAt, verdict.ResetSource = when, domain.ResetSourceBody
		} else if when := firstReset(genericResets, f.Body); !when.IsZero() {
			verdict.ResetAt, verdict.ResetSource = when, domain.ResetSourceBody
		} else {
			verdict.applyRetryAfter(f)
		}
		return verdict
	}

	verdict := FailureVerdict{Class: domain.FailureClassForStatus(f.Status)}
	if when := firstReset(genericResets, f.Body); !when.IsZero() {
		verdict.ResetAt, verdict.ResetSource = when, domain.ResetSourceBody
	} else {
		verdict.applyRetryAfter(f)
	}
	return verdict
}

// applyRetryAfter falls back to the header the vendor sent. It reports the wait
// rather than an instant, because a Retry-After is a duration the vendor guessed
// for its own queue and the park still holds it to the class ceiling.
func (v *FailureVerdict) applyRetryAfter(f Failure) {
	wait := RetryAfter(f.Header)
	if wait <= 0 {
		return
	}
	v.RetryAfter = wait
	v.ResetSource = domain.ResetSourceHeader
}
