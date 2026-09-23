// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_consumer_test.go
// @for       The guard that the two capability consumers read one table.
//
// @uses      testing.
// @reason    SPEC-API-001 §7.8 refuses a vision adapter whose model cannot read
//
//	images, and §7.6 filters the catalog by the same two names. Draft 017
//	§4.4 found the consequence of those being two sources: the catalog
//	answered zero rows while the adapter answered correctly, and neither
//	was wrong on its own. This test fails when a second table reappears.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package registry

import "testing"

// TestCapabilities_OneTableServesBothConsumers pins the property draft 017 §4.4
// asks for: the vision adapter's predicate and the catalog's resolver are one
// decision, so the two cannot disagree about a model.
//
// This is a guard, not a defect reproduction. It passed the moment
// capability_resolve.go landed, because VisionCapable delegates to Capabilities;
// it exists so that the next edit which reintroduces a local walk of the table
// fails here by name rather than in the panel, as a filter that answers nothing.
func TestCapabilities_OneTableServesBothConsumers(t *testing.T) {
	// The ids span every shape the table distinguishes: a family pattern, a
	// narrower pattern that shadows it, an exact-id exception, the floor, and
	// the one model that declares no tool calling.
	ids := []string{
		"claude-opus-4-8",
		"claude-3-haiku-20240307",
		"gpt-5.5",
		"gpt-5.5-image",
		"gpt-4o",
		"gpt-image-1",
		"gemini-3-pro-preview",
		"glm-4.6v",
		"vision-model",
		"deepseek-v4-flash",
		"qwen3-coder-480b",
		"unknown-model",
	}
	providers := []string{"", "openai", "claude", "gemini", "deepseek", "glm", "qwen"}
	for _, id := range ids {
		for _, provider := range providers {
			resolved := Capabilities(provider, id)
			if got := VisionCapable(id); got != resolved.Vision {
				t.Fatalf("VisionCapable(%q) = %v but Capabilities(%q, %q).Vision = %v; the adapter and the catalog read different tables",
					id, got, provider, id, resolved.Vision)
			}
		}
	}
}
