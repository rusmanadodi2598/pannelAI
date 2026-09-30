// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/stop_sequence_test.go
// @for       The stop-sequence guard: what it cuts, what it withholds, and what it
//
//	hands back once an answer ends.
//
// @uses      testing, internal/dataplane stopGuard.
// @reason    The guard's whole difficulty is a marker that arrives in pieces, and a
//
//	table of fragments is the only way to pin that no part of a marker is
//	ever shown while no real tail text is lost.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-30
package dataplane

import (
	"strings"
	"testing"
)

func TestStopGuard_CutsAtTheMarker(t *testing.T) {
	guard := newStopGuard([]string{"STOPHERE"})
	got := guard.write("before STOPHERE after")
	got += guard.write("more after")
	if got != "before " {
		t.Fatalf("write() = %q, want the text before the marker only", got)
	}
	if !guard.stopped() {
		t.Fatal("stopped() = false, want true once the marker passed through")
	}
	if tail := guard.flush(); tail != "" {
		t.Fatalf("flush() = %q, want nothing after a cut", tail)
	}
}

func TestStopGuard_CutsAMarkerSplitAcrossFragments(t *testing.T) {
	guard := newStopGuard([]string{"STOPHERE"})
	// Every fragment is shown as it arrives, so a partial marker must be held
	// rather than emitted: showing "STOP" then "HERE" is the leak.
	var shown strings.Builder
	shown.WriteString(guard.write("A"))
	shown.WriteString(guard.write("B STOP"))
	shown.WriteString(guard.write("HERE"))
	shown.WriteString(guard.write(" C"))
	if shown.String() != "AB " {
		t.Fatalf("fragments shown = %q, want %q", shown.String(), "AB ")
	}
	if !guard.stopped() {
		t.Fatal("stopped() = false, want the split marker recognised")
	}
}

func TestStopGuard_ReleasesATailThatWasNotAMarker(t *testing.T) {
	guard := newStopGuard([]string{"STOPHERE"})
	shown := guard.write("the end is near STOP")
	if shown != "the end is near " {
		t.Fatalf("write() = %q, want the partial marker held back", shown)
	}
	if tail := guard.flush(); tail != "STOP" {
		t.Fatalf("flush() = %q, want the held text released at the end", tail)
	}
	if guard.stopped() {
		t.Fatal("stopped() = true, want false: no complete marker arrived")
	}
}

func TestStopGuard_TakesTheEarliestOfSeveralMarkers(t *testing.T) {
	guard := newStopGuard([]string{"ZZ", "AA"})
	if shown := guard.write("x AAno ZZno"); shown != "x " {
		t.Fatalf("write() = %q, want the cut at the earliest marker", shown)
	}
}

func TestStopGuard_ShortestMarkerWinsItsPosition(t *testing.T) {
	// "AB" starts before "ABC" at the same offset, and either cut leaves the same
	// visible text; the guard must not miss the earlier of two overlapping hits.
	guard := newStopGuard([]string{"ABC", "AB"})
	if shown := guard.write("12AB34"); shown != "12" {
		t.Fatalf("write() = %q, want the cut at the first marker position", shown)
	}
}

func TestStopGuard_WithoutSequencesPassesTextThrough(t *testing.T) {
	if guard := newStopGuard(nil); guard != nil {
		t.Fatal("newStopGuard(nil) built a guard, want none")
	}
	if guard := newStopGuard([]string{""}); guard != nil {
		t.Fatal("newStopGuard built a guard for an empty marker, want none")
	}
	// A nil guard is the no-sequence case and must still answer every call.
	var guard *stopGuard
	if shown := guard.write("everything"); shown != "everything" {
		t.Fatalf("write() = %q, want the text unchanged", shown)
	}
	if guard.flush() != "" || guard.stopped() {
		t.Fatal("a nil guard reported a cut")
	}
}

func TestCutAtStop_TrimsAFoldedAnswer(t *testing.T) {
	cases := []struct {
		text     string
		want     string
		wantMark string
		wantCut  bool
		sequence []string
	}{
		{text: "keep CUT drop", want: "keep ", wantMark: "CUT", wantCut: true, sequence: []string{"CUT"}},
		{text: "no marker here", want: "no marker here", wantCut: false, sequence: []string{"CUT"}},
		{text: "CUT at the very start", want: "", wantMark: "CUT", wantCut: true, sequence: []string{"CUT"}},
		{text: "nothing to cut", want: "nothing to cut", wantCut: false, sequence: nil},
		// Two markers in play: the earliest one ends the answer, and it is the one
		// the Anthropic wire reports back as `stop_sequence`.
		{text: "bb then aa", want: "", wantMark: "bb", wantCut: true, sequence: []string{"aa", "bb"}},
	}
	for _, tc := range cases {
		got, matched, cut := cutAtStop(tc.text, tc.sequence)
		if got != tc.want || cut != tc.wantCut || matched != tc.wantMark {
			t.Fatalf("cutAtStop(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.text, got, matched, cut, tc.want, tc.wantMark, tc.wantCut)
		}
	}
}
