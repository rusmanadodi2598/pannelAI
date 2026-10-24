// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/builder_test.go
// @for       The builder's two-attempt retry and its fallback, driven by a fake gateway.
// @uses      encoding/json, errors, strings, testing.
// @reason    The fallback is the last word before a credit is held, so the retry count and the fallback label are pinned here against a transport that never reaches the network.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

// fakeGateway returns one canned reply per call and counts them.
type fakeGateway struct {
	replies []string
	errs    []error
	usages  []masterprompt.Usage
	calls   int
	last    masterprompt.ChatRequest
	usage   masterprompt.Usage
}

func (f *fakeGateway) Complete(_ context.Context, req masterprompt.ChatRequest) (masterprompt.ChatResult, error) {
	f.last = req
	index := f.calls
	f.calls++
	if index < len(f.errs) && f.errs[index] != nil {
		return masterprompt.ChatResult{}, f.errs[index]
	}
	if index >= len(f.replies) {
		return masterprompt.ChatResult{}, errors.New("the fake gateway ran out of replies")
	}
	usage := f.usage
	if index < len(f.usages) {
		usage = f.usages[index]
	}
	return masterprompt.ChatResult{ModelID: "fake-model", Content: f.replies[index], Usage: usage}, nil
}

func validImageJSON(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(imageDocument(t))
	if err != nil {
		t.Fatalf("marshal the fixture: %v", err)
	}
	return string(raw)
}

func TestBuilder_GivenTransportVariations_ThenItBuildsOrFallsBack(t *testing.T) {
	valid := validImageJSON(t)
	blank := `{}`
	transport := errors.New("gateway refused the connection")

	cases := []struct {
		name        string
		replies     []string
		errs        []error
		wantBuilder string
		wantCalls   int
	}{
		{
			name:        "a valid first attempt builds and stops",
			replies:     []string{valid},
			wantBuilder: "fake-model",
			wantCalls:   1,
		},
		{
			name:        "an invalid first attempt is retried once",
			replies:     []string{blank, valid},
			wantBuilder: "fake-model",
			wantCalls:   2,
		},
		{
			name:        "two invalid attempts fall back",
			replies:     []string{blank, blank},
			wantBuilder: "fallback",
			wantCalls:   2,
		},
		{
			name:        "two transport errors fall back",
			errs:        []error{transport, transport},
			wantBuilder: "fallback",
			wantCalls:   2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gw := &fakeGateway{replies: tc.replies, errs: tc.errs}
			b := masterprompt.NewBuilder(gw, "fake-model", 700, 2000)

			result, err := b.Build(context.Background(), masterprompt.BuildRequest{
				Modality: "image",
				UserText: "create a hero image of an orange cat riding a skateboard",
				Defaults: masterprompt.Defaults{AspectRatio: "16:9", Style: "cinematic photo"},
			})
			if err != nil {
				t.Fatalf("Build() error = %v, want none", err)
			}
			if result.Builder != tc.wantBuilder {
				t.Fatalf("Builder = %q, want %q", result.Builder, tc.wantBuilder)
			}
			if gw.calls != tc.wantCalls {
				t.Fatalf("gateway calls = %d, want %d", gw.calls, tc.wantCalls)
			}
			if result.Document.Version != masterprompt.Version {
				t.Fatalf("version = %q, want %q", result.Document.Version, masterprompt.Version)
			}
			// The retry hands the failed validation error back to the model.
			if tc.wantCalls == 2 && tc.wantBuilder == "fallback" && len(tc.replies) == 2 {
				if !strings.Contains(gw.last.User, "failed validation") {
					t.Fatalf("retry user message = %q, want it to carry the correction", gw.last.User)
				}
			}
		})
	}
}

func TestBuilder_GivenEmptyText_ThenItIsRefusedWithoutACall(t *testing.T) {
	gw := &fakeGateway{}
	b := masterprompt.NewBuilder(gw, "fake-model", 700, 2000)

	_, err := b.Build(context.Background(), masterprompt.BuildRequest{
		Modality: "image",
		UserText: "   ",
	})

	var promptErr *masterprompt.Error
	if !errors.As(err, &promptErr) {
		t.Fatalf("Build() error = %v, want a prompt error", err)
	}
	if promptErr.Code != masterprompt.CodeEmpty {
		t.Fatalf("code = %q, want %q", promptErr.Code, masterprompt.CodeEmpty)
	}
	if gw.calls != 0 {
		t.Fatalf("gateway calls = %d, want none for an empty draft", gw.calls)
	}
}

func TestBuilder_GivenASuccess_ThenUsageAndTemperatureAreRecorded(t *testing.T) {
	gw := &fakeGateway{replies: []string{validImageJSON(t)}, usage: masterprompt.Usage{PromptTokens: 120, CompletionTokens: 300, TotalTokens: 420}}
	b := masterprompt.NewBuilder(gw, "fake-model", 700, 2000)

	result, err := b.Build(context.Background(), masterprompt.BuildRequest{
		Modality: "image",
		UserText: "create a hero image of an orange cat",
		Defaults: masterprompt.Defaults{AspectRatio: "16:9", Style: "cinematic photo"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v, want none", err)
	}
	if result.Usage.TotalTokens != 420 {
		t.Fatalf("usage total = %d, want 420", result.Usage.TotalTokens)
	}
	if result.Builder != "fake-model" {
		t.Fatalf("Builder = %q, want the model id on a clean build", result.Builder)
	}
	if gw.last.Temperature != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", gw.last.Temperature)
	}
}

func TestBuilder_GivenURLOnlyText_ThenItIsRefusedWithoutACall(t *testing.T) {
	gw := &fakeGateway{}
	b := masterprompt.NewBuilder(gw, "fake-model", 700, 2000)

	_, err := b.Build(context.Background(), masterprompt.BuildRequest{
		Modality: "image",
		UserText: "https://example.com/street",
	})

	var promptErr *masterprompt.Error
	if !errors.As(err, &promptErr) {
		t.Fatalf("Build() error = %v, want a prompt error", err)
	}
	if promptErr.Code != masterprompt.CodeEmpty {
		t.Fatalf("code = %q, want %q for a link-only draft", promptErr.Code, masterprompt.CodeEmpty)
	}
	if gw.calls != 0 {
		t.Fatalf("gateway calls = %d, want none", gw.calls)
	}
}

func TestBuilder_GivenUnknownDefaults_ThenTheyAreClampedToTheVocabulary(t *testing.T) {
	gw := &fakeGateway{replies: []string{validImageJSON(t)}}
	b := masterprompt.NewBuilder(gw, "fake-model", 700, 2000)

	_, err := b.Build(context.Background(), masterprompt.BuildRequest{
		Modality: "image",
		UserText: "create a hero image of an orange cat",
		Defaults: masterprompt.Defaults{AspectRatio: "99:99", Style: "some free text style"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v, want none", err)
	}
	if !strings.Contains(gw.last.System, "Default aspect ratio: 16:9") {
		t.Fatalf("system = %q, want the unknown aspect clamped to 16:9", gw.last.System)
	}
	if !strings.Contains(gw.last.System, "Default style: cinematic photo") {
		t.Fatalf("system = %q, want the unknown style clamped to a known medium", gw.last.System)
	}
}

func TestBuilder_GivenATwoAttemptSuccess_ThenUsageAccumulates(t *testing.T) {
	valid := validImageJSON(t)
	gw := &fakeGateway{
		replies: []string{"{}", valid},
		usages: []masterprompt.Usage{
			{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			{PromptTokens: 15, CompletionTokens: 25, TotalTokens: 40},
		},
	}
	b := masterprompt.NewBuilder(gw, "fake-model", 700, 2000)

	result, err := b.Build(context.Background(), masterprompt.BuildRequest{
		Modality: "image",
		UserText: "create a hero image of an orange cat",
		Defaults: masterprompt.Defaults{AspectRatio: "16:9", Style: "cinematic photo"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v, want none", err)
	}
	if result.Usage.TotalTokens != 70 {
		t.Fatalf("usage total = %d, want both attempts summed to 70", result.Usage.TotalTokens)
	}
}
