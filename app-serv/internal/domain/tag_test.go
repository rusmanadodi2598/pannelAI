// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/tag_test.go
// @for       The Tag value object: its canonical spelling, its alphabet, and the bounds a row's set obeys.
// @uses      fmt, strings, testing.
// @reason    A tag is the address a bulk action is sent to, so the cases that silently change which rows an action reaches are the ones worth pinning: two spellings of one label, an over-long label refused rather than cut, and a set that stops growing at the bound instead of at the index.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-10-16
package domain

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewTag(t *testing.T) {
	cases := []struct {
		name    string
		given   string
		want    string
		wantErr bool
	}{
		{"kept as written", "free-tier", "free-tier", false},
		{"case folded to lowercase", "Event_Week", "event_week", false},
		{"padding trimmed", "  parked-24h  ", "parked-24h", false},
		{"a single digit", "7", "7", false},
		{"empty after trimming", "   ", "", true},
		{"leading dash", "-free", "", true},
		{"leading underscore", "_free", "", true},
		{"a dot is not in the alphabet", "free.tier", "", true},
		{"a space inside", "free tier", "", true},
		{"one past the ceiling", "t" + repeatRun(48), "", true},
		{"exactly the ceiling", "t" + repeatRun(47), "t" + repeatRun(47), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tag, err := NewTag(tc.given)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NewTag(%q) = %q, want a refusal", tc.given, tag)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTag(%q) error = %v", tc.given, err)
			}
			if tag.String() != tc.want {
				t.Fatalf("NewTag(%q) = %q, want %q", tc.given, tag, tc.want)
			}
		})
	}
}

func TestNormalizeTagsFoldsDuplicatesBeforeCountingTheBound(t *testing.T) {
	cases := []struct {
		name    string
		given   []string
		want    []string
		wantErr bool
	}{
		{"nothing in, nothing out", nil, nil, false},
		{"one spelling of one label", []string{"Free", " free ", "FREE"}, []string{"free"}, false},
		{"distinct labels kept in order", []string{"b-free", "a-free"}, []string{"b-free", "a-free"}, false},
		{"the bound is met, not exceeded", repeatDistinct(16, "tag-"), repeatDistinct(16, "tag-"), false},
		{"one past the bound after folding", append(repeatDistinct(16, "tag-"), "extra"), nil, true},
		{"a bad member refuses the set", []string{"good", "Bad Space"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tags, err := NormalizeTags(tc.given)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeTags(%v) = %v, want a refusal", tc.given, tags)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeTags(%v) error = %v", tc.given, err)
			}
			if len(tags) != len(tc.want) {
				t.Fatalf("NormalizeTags(%v) = %v, want %v", tc.given, tags, tc.want)
			}
			for i, want := range tc.want {
				if tags[i].String() != want {
					t.Fatalf("tag %d = %q, want %q", i, tags[i], want)
				}
			}
		})
	}
}

// TestTagSetCopyIsolatesTheAggregate proves the read cannot hand out the aggregate's
// own storage: appending to what Tags() returned would otherwise grow a set the row
// never stored.
func TestTagSetCopyIsolatesTheAggregate(t *testing.T) {
	set := TagSet{}
	if _, err := set.Add("free-tier"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	copied := append(set.Copy(), Tag("ghost"))
	if len(copied) != 2 {
		t.Fatalf("the copy holds %d tags, want the appended one to land on the copy", len(copied))
	}
	if len(set) != 1 {
		t.Fatalf("the aggregate holds %d tags, want the one it was given", len(set))
	}
}

func repeatRun(n int) string {
	return strings.Repeat("x", n)
}

func repeatDistinct(n int, prefix string) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("%s%02d", prefix, i))
	}
	return out
}
