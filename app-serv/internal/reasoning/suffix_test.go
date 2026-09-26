// Package reasoning ports the reference's thinking normalization: the
// level-to-budget maps, the model-name suffix, and the per-format application
// that turns a reasoning intent into the field an upstream reads.
//
// @file      internal/reasoning/suffix_test.go
// @for       The model(level) suffix: its shapes, its vocabulary, and the
//
//	strings it must leave untouched.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.15 ports the reference's per-model control, and the
//
//	suffix is a client-supplied string on the routing key: a parse that
//	strips too much renames the model and one that strips too little sends
//	the suffix upstream. Both are silent, so the shapes are pinned here.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-26
package reasoning

import "testing"

// TestParseSuffix pins every shape the reference's parser accepts, and the two
// shapes it must not touch.
func TestParseSuffix(t *testing.T) {
	cases := []struct {
		name  string
		model string
		clean string
		want  *Suffix
	}{
		{"no suffix", "gpt-5.5", "gpt-5.5", nil},
		{"a level", "gpt-5.5(high)", "gpt-5.5", &Suffix{Mode: "level", Level: "high"}},
		{"an upper-cased level", "gpt-5.5(HIGH)", "gpt-5.5", &Suffix{Mode: "level", Level: "high"}},
		{"a padded level", "gpt-5.5( high )", "gpt-5.5", &Suffix{Mode: "level", Level: "high"}},
		{"none", "gpt-5.5(none)", "gpt-5.5", &Suffix{Mode: "none"}},
		{"off", "gpt-5.5(off)", "gpt-5.5", &Suffix{Mode: "none"}},
		{"auto", "gpt-5.5(auto)", "gpt-5.5", &Suffix{Mode: "auto"}},
		{"ultra", "gpt-5.5(ultra)", "gpt-5.5", &Suffix{Mode: "level", Level: "ultra"}},
		{"a budget", "gpt-5.5(8192)", "gpt-5.5", &Suffix{Mode: "budget", Budget: 8192}},
		{"a zero budget", "gpt-5.5(0)", "gpt-5.5", &Suffix{Mode: "budget", Budget: 0}},
		{"an unknown word", "gpt-5.5(bogus)", "gpt-5.5", nil},
		{"a provider-qualified id", "openai/gpt-5.5(medium)", "openai/gpt-5.5", &Suffix{Mode: "level", Level: "medium"}},
		{"trailing space", "gpt-5.5(low) ", "gpt-5.5", &Suffix{Mode: "level", Level: "low"}},
		{"the last group wins", "gpt-5.5(a)(max)", "gpt-5.5(a)", &Suffix{Mode: "level", Level: "max"}},
		{"parentheses inside the id", "model(a)extra", "model(a)extra", nil},
		{"an empty group", "gpt-5.5()", "gpt-5.5()", nil},
		{"a negative number", "gpt-5.5(-5)", "gpt-5.5", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clean, got := ParseSuffix(tc.model)
			if clean != tc.clean {
				t.Fatalf("ParseSuffix(%q) clean = %q, want %q", tc.model, clean, tc.clean)
			}
			if !sameSuffix(got, tc.want) {
				t.Fatalf("ParseSuffix(%q) override = %+v, want %+v", tc.model, got, tc.want)
			}
		})
	}
}

// TestStripSuffix pins the resolver's view: the group goes, whatever it holds,
// so an upstream never receives a suffix.
func TestStripSuffix(t *testing.T) {
	cases := map[string]string{
		"gpt-5.5":           "gpt-5.5",
		"gpt-5.5(high)":     "gpt-5.5",
		"gpt-5.5(bogus)":    "gpt-5.5",
		"gpt-5.5()":         "gpt-5.5()",
		"model(a)extra":     "model(a)extra",
		"openai/gpt-5(max)": "openai/gpt-5",
		"gpt-5.5( high ) ":  "gpt-5.5",
		"a(b)(c)":           "a(b)",
	}
	for model, want := range cases {
		if got := StripSuffix(model); got != want {
			t.Fatalf("StripSuffix(%q) = %q, want %q", model, got, want)
		}
	}
}

// sameSuffix compares two optional overrides, so a nil expectation is not
// confused with an override that happens to be zero-valued.
func sameSuffix(got, want *Suffix) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}
