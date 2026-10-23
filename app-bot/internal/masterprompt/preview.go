// Package masterprompt builds and compiles the six-section master prompt.
//
// @file      internal/masterprompt/preview.go
// @for       The preview message and its inline keyboard.
// @uses      fmt, strings.
// @reason    Generating from the preview is the only path that holds credits, so the message states the cost and the keyboard carries an opaque draft id rather than the prompt.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-23
package masterprompt

import (
	"fmt"
	"strings"
)

// Button is one inline keyboard entry.
type Button struct {
	Label string
	Data  string
}

// PreviewView is the message body plus its keyboard.
type PreviewView struct {
	Text    string
	Buttons []Button
}

// PreviewOptions are the per-draft details the document does not carry.
type PreviewOptions struct {
	DraftID string
	Builder string
	Credits int
}

// RenderPreview builds the preview shown before any credit is held.
func RenderPreview(doc Document, opts PreviewOptions) (PreviewView, error) {
	lines := []string{
		"MASTER PROMPT",
		"Role: " + doc.Role,
		"Task: " + doc.Task,
		"Context: " + contextLine(doc.Context),
		"Reasoning: " + truncate(doc.Reasoning, reasoningView),
		"Stop: " + stopLine(doc.StopConditions),
		"Output: " + outputLine(doc.Output),
	}
	if opts.Builder == "fallback" {
		lines = append(lines, "Builder: fallback")
	}
	text := strings.Join(lines, "\n")
	if runeLen(text) > budgetPreview {
		return PreviewView{}, &Error{
			Code:    CodePreview,
			Message: fmt.Sprintf("preview is %d characters, limit is %d", runeLen(text), budgetPreview),
		}
	}

	buttons, err := keyboard(doc, opts)
	if err != nil {
		return PreviewView{}, err
	}
	return PreviewView{Text: text, Buttons: buttons}, nil
}

func contextLine(context Context) string {
	parts := []string{
		phraseValue(context.Composition.Shot) + " shot",
		phraseValue(context.Composition.Angle),
		context.Composition.AspectRatio,
		context.Lighting,
		context.Style.Medium,
	}
	if context.Style.Mood != "" {
		parts = append(parts, context.Style.Mood)
	}
	return joinPresent(parts)
}

func stopLine(stop StopConditions) string {
	negatives := dedupe(append(append([]string{}, stop.Negative...), baselineNegative...))
	return fmt.Sprintf("avoid %s; max %d chars", strings.Join(negatives, ", "), stop.MaxCharacters)
}

func outputLine(output Output) string {
	return fmt.Sprintf("%d %s, %s, %s", output.Count, output.Asset, output.AspectRatio, output.Format)
}

func joinPresent(parts []string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if clean := NormalizeText(part); clean != "" {
			kept = append(kept, clean)
		}
	}
	return strings.Join(kept, ", ")
}

// keyboard keeps callback data under Telegram's 64 byte limit: the draft id is
// the only variable part, and the prompt never travels through a button.
func keyboard(doc Document, opts PreviewOptions) ([]Button, error) {
	buttons := []Button{
		{Label: generateLabel(doc.Modality, opts.Credits), Data: callbackData("gen", opts.DraftID)},
		{Label: "Aspect", Data: callbackData("aspect", opts.DraftID)},
		{Label: "Style", Data: callbackData("style", opts.DraftID)},
		{Label: "Edit text", Data: callbackData("edit", opts.DraftID)},
		{Label: "Cancel", Data: callbackData("cancel", opts.DraftID)},
	}
	for _, button := range buttons {
		if len(button.Data) > 64 {
			return nil, &Error{Code: CodePreview, Message: "callback data exceeds 64 bytes"}
		}
	}
	return buttons, nil
}

func generateLabel(modality string, credits int) string {
	unit := "credits"
	if credits == 1 {
		unit = "credit"
	}
	return fmt.Sprintf("Generate %s - %d %s", modality, credits, unit)
}

func callbackData(action, draftID string) string {
	return "mpv2:" + action + ":" + draftID
}
