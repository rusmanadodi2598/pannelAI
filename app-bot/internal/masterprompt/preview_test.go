// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/preview_test.go
// @for       The preview message and its inline keyboard.
// @uses      testing, strings, errors.
// @reason    The preview is the last screen before credits are held, so its cap, its honest fallback label, and its button payloads are pinned.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

func TestRenderPreview_GivenADocument_ThenTheMessageAndKeyboardAreComplete(t *testing.T) {
	doc := decodeImage(t)

	view, err := masterprompt.RenderPreview(doc, masterprompt.PreviewOptions{
		DraftID: "draft-7f3c2b1a",
		Builder: "model",
		Credits: 1,
	})
	if err != nil {
		t.Fatalf("RenderPreview() error = %v, want nil", err)
	}

	for _, want := range []string{"MASTER PROMPT", "Role: ", "Task: ", "Context: ", "Reasoning: ", "Stop: ", "Output: "} {
		if !strings.Contains(view.Text, want) {
			t.Fatalf("preview text = %q, want it to contain %q", view.Text, want)
		}
	}
	if strings.Contains(view.Text, "Builder: fallback") {
		t.Fatalf("preview text = %q, must not claim a fallback for a model build", view.Text)
	}
	if len(view.Buttons) != 5 {
		t.Fatalf("buttons = %d, want 5", len(view.Buttons))
	}
	if view.Buttons[0].Label != "Generate image - 1 credit" {
		t.Fatalf("first button = %q, want the generate action with its cost", view.Buttons[0].Label)
	}
	for _, button := range view.Buttons {
		if len(button.Data) > 64 {
			t.Fatalf("button %q carries %d bytes of callback data, want at most 64", button.Label, len(button.Data))
		}
		if !strings.Contains(button.Data, "draft-7f3c2b1a") {
			t.Fatalf("button %q data = %q, want the draft id", button.Label, button.Data)
		}
	}
}

func TestRenderPreview_GivenLongReasoning_ThenTheLineIsTruncated(t *testing.T) {
	doc := decodeImage(t)
	doc.Reasoning = strings.Repeat("warm street scene reads faster ", 20)

	view, err := masterprompt.RenderPreview(doc, masterprompt.PreviewOptions{DraftID: "draft-1", Builder: "model", Credits: 1})
	if err != nil {
		t.Fatalf("RenderPreview() error = %v, want nil", err)
	}

	line := reasoningLine(t, view.Text)
	if got := len([]rune(strings.TrimPrefix(line, "Reasoning: "))); got != 160 {
		t.Fatalf("reasoning runes = %d, want 160", got)
	}
}

func TestRenderPreview_GivenFallbackBuild_ThenTheLabelIsHonest(t *testing.T) {
	doc := decodeImage(t)

	view, err := masterprompt.RenderPreview(doc, masterprompt.PreviewOptions{DraftID: "draft-2", Builder: "fallback", Credits: 1})
	if err != nil {
		t.Fatalf("RenderPreview() error = %v, want nil", err)
	}
	if !strings.Contains(view.Text, "Builder: fallback") {
		t.Fatalf("preview text = %q, want the fallback label", view.Text)
	}
}

func TestRenderPreview_GivenAnOversizedMessage_ThenItIsRefused(t *testing.T) {
	doc := decodeImage(t)
	doc.Task = strings.Repeat("panjang ", 500)

	_, err := masterprompt.RenderPreview(doc, masterprompt.PreviewOptions{DraftID: "draft-3", Builder: "model", Credits: 1})

	var promptErr *masterprompt.Error
	if !errors.As(err, &promptErr) {
		t.Fatalf("RenderPreview() error = %v, want a prompt error", err)
	}
	if promptErr.Code != masterprompt.CodePreview {
		t.Fatalf("code = %q, want %q", promptErr.Code, masterprompt.CodePreview)
	}
}

func reasoningLine(t *testing.T, text string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Reasoning: ") {
			return line
		}
	}
	t.Fatalf("preview text = %q, want a reasoning line", text)
	return ""
}
