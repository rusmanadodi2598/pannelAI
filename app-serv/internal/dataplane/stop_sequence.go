// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stop_sequence.go
// @for       Cutting an answer at a `stop` sequence the caller asked for, whichever
//
//	of the two served shapes the answer arrives in.
//
// @uses      strings.
// @reason    The OpenAI wire makes `stop` a promise the gateway keeps: a client that
//
//	asks to stop at a marker expects the marker and everything after it to be
//	absent. Some upstreams on that wire ignore the member and answer in full,
//	so the promise would silently not hold. Measured live against
//	codebuddy-intl/deepseek-v4.1-flash on 2026-09-30: the answer was
//	byte-identical with and without a stop sequence. The cut therefore happens
//	here, on the way to the client, and a held-back tail means no fragment of a
//	marker ever reaches the client even when the vendor splits it across frames.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-30
package dataplane

import "strings"

// stopGuard holds the sequences one caller asked to stop at and the text not yet
// safe to show, because it could still turn out to be the start of a sequence.
//
// A stream delivers content in fragments, so a vendor that does stop at the marker
// can split it across two of them ("STOP" then "HERE"). Matching only within one
// fragment would let the marker through in pieces, which is the exact thing the
// caller asked not to see; the held-back tail is what closes that gap.
type stopGuard struct {
	sequences []string
	pending   string
	// hit reports a sequence already found: everything after it belongs to the
	// vendor's draft, not to the answer.
	hit bool
	// matched is the sequence that ended the answer, which the Anthropic wire
	// reports back to the client as `stop_sequence`.
	matched string
}

// newStopGuard builds the guard for one call. No sequences means nothing to cut
// at, which is the same request as one that omitted the field.
func newStopGuard(sequences []string) *stopGuard {
	kept := make([]string, 0, len(sequences))
	for _, sequence := range sequences {
		if sequence != "" {
			kept = append(kept, sequence)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return &stopGuard{sequences: kept}
}

// write takes one content fragment and returns the text the caller may see now.
// The rest stays held back until it is either proven to be a marker or the answer
// ends.
func (g *stopGuard) write(fragment string) string {
	if g == nil {
		return fragment
	}
	if g.hit {
		return ""
	}
	buffered := g.pending + fragment
	g.pending = ""
	if at, sequence := firstStopSequence(buffered, g.sequences); at >= 0 {
		g.hit = true
		g.matched = sequence
		return buffered[:at]
	}
	hold := heldBackLen(buffered, g.sequences)
	g.pending = buffered[len(buffered)-hold:]
	return buffered[:len(buffered)-hold]
}

// flush releases what the guard was holding once the answer ends. A tail held back
// to the end was never a marker, and dropping it would cost the caller real text.
func (g *stopGuard) flush() string {
	if g == nil {
		return ""
	}
	tail := g.pending
	g.pending = ""
	return tail
}

// stopped reports whether the answer was cut, which is what makes the finish
// reason "stop" rather than whatever the vendor went on to say.
func (g *stopGuard) stopped() bool {
	return g != nil && g.hit
}

// sequence names the marker that ended the answer, or "" when nothing was cut.
func (g *stopGuard) sequence() string {
	if g == nil {
		return ""
	}
	return g.matched
}

// cutAtStop trims a whole answer to before its first marker, reporting the marker
// that matched and whether one did. It serves the folded and one-body shapes,
// where the text is complete before anyone reads it.
func cutAtStop(text string, sequences []string) (trimmed string, matched string, cut bool) {
	at, sequence := firstStopSequence(text, sequences)
	if at < 0 {
		return text, "", false
	}
	return text[:at], sequence, true
}

// firstStopSequence returns the earliest position of any sequence in text and the
// sequence that holds it, or -1 when none appears.
func firstStopSequence(text string, sequences []string) (int, string) {
	earliest, matched := -1, ""
	for _, sequence := range sequences {
		at := strings.Index(text, sequence)
		if at >= 0 && (earliest < 0 || at < earliest) {
			earliest, matched = at, sequence
		}
	}
	return earliest, matched
}

// heldBackLen returns the length of the longest tail of text that could still grow
// into a sequence, so only that much is withheld.
func heldBackLen(text string, sequences []string) int {
	longest := 0
	for _, sequence := range sequences {
		// A complete sequence never needs holding: the caller has been told the
		// answer stops there, so the search reports it rather than this.
		limit := len(sequence) - 1
		if limit > len(text) {
			limit = len(text)
		}
		for length := limit; length > longest; length-- {
			if strings.HasSuffix(text, sequence[:length]) {
				longest = length
				break
			}
		}
	}
	return longest
}
