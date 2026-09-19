// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/fusion_prompt_test.go
// @for       Reading a panel member's prose back out of its answer, and the
//
//	directive the judge receives.
//
// @uses      testing, strings, internal/schema.
// @reason    Both are decidable without a network, so a table pins every
//
//	wire-format spelling here rather than in a fusion test that would
//	report a regression as a mysterious panel failure. The request
//	shaping these two feed is pinned beside them, in fusion_shape_test.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestAnswerTextReadsBothWireFormats pins the judge's input: the prose is read
// out of whichever format the client asked for, and an answer with nothing to
// read yields an empty string rather than a decode failure.
func TestAnswerTextReadsBothWireFormats(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		format schema.DataPlaneFormat
		want   string
	}{
		{
			name:   "an OpenAI string answer",
			format: schema.FormatOpenAI,
			body:   `{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`,
			want:   "hello",
		},
		{
			name:   "an OpenAI parts answer flattens to text",
			format: schema.FormatOpenAI,
			body: `{"choices":[{"message":{"role":"assistant","content":[` +
				`{"type":"text","text":"one"},{"type":"text","text":"two"}]}}]}`,
			want: "one\ntwo",
		},
		{
			name:   "an Anthropic text-block answer joins its blocks",
			format: schema.FormatAnthropic,
			body:   `{"content":[{"type":"text","text":"one"},{"type":"text","text":"two"}]}`,
			want:   "one\ntwo",
		},
		{
			name:   "an Anthropic thinking block is not prose",
			format: schema.FormatAnthropic,
			body:   `{"content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"answer"}]}`,
			want:   "answer",
		},
		{
			name:   "an OpenAI answer with no choices reads empty",
			format: schema.FormatOpenAI,
			body:   `{"choices":[]}`,
			want:   "",
		},
		{
			name:   "an empty body reads empty",
			format: schema.FormatOpenAI,
			body:   "",
			want:   "",
		},
		{
			name:   "an unreadable body reads empty",
			format: schema.FormatAnthropic,
			body:   `not json`,
			want:   "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := answerText([]byte(testCase.body), testCase.format); got != testCase.want {
				t.Fatalf("answerText() = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestBuildJudgePromptNumbersAndAnonymizesSources pins the directive: every
// source is numbered in member order, the texts appear verbatim, and the judge
// is told how many models answered.
func TestBuildJudgePromptNumbersAndAnonymizesSources(t *testing.T) {
	cases := []struct {
		name    string
		answers []fusionAnswer
		want    []string
	}{
		{
			name:    "two sources",
			answers: []fusionAnswer{{text: "alpha prose"}, {text: "beta prose"}},
			want:    []string{"2 expert models", "[Source 1]\nalpha prose", "[Source 2]\nbeta prose"},
		},
		{
			name:    "three sources",
			answers: []fusionAnswer{{text: "one"}, {text: "two"}, {text: "three"}},
			want:    []string{"3 expert models", "[Source 1]\none", "[Source 2]\ntwo", "[Source 3]\nthree"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			prompt := buildJudgePrompt(testCase.answers)
			if !strings.Contains(prompt, "You are the JUDGE") {
				t.Fatalf("prompt = %q, want the judge directive", prompt)
			}
			last := -1
			for _, want := range testCase.want {
				at := strings.Index(prompt, want)
				if at < 0 {
					t.Fatalf("prompt = %q, want it to contain %q", prompt, want)
				}
				if at < last {
					t.Fatalf("prompt = %q, want %q after the previous member", prompt, want)
				}
				last = at
			}
		})
	}
}
