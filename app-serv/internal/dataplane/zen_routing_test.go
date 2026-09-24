// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/zen_routing_test.go
// @for       The multi-endpoint rule against the real registry: every model the
// //
//
//	entry declares reaches a URL, and the claude-only ids reach the
//	messages endpoint.
//
// @uses      testing, internal/provider, internal/registry.
// @reason    Draft 029 F2 measured the broken shape on opencode-go
// //
//
//	(".../chat/completions/zen/v1/messages") and the first fix for it
//	refused 43 of opencode-zen's models instead, because the provider's
//	default wire is openai while those models declare claude only. Both
//	failures are about the same rule, so this test states the property
//	that rules them out: every declared model builds a URL, and a model
//	that supports the claude wire is sent to the messages endpoint when
//	the request is translated into claude.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/provider"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestRegistry_MultiEndpointModelsAllBuildAURL pins the property that keeps a
// model from being unroutable: every model every multi-endpoint entry declares
// produces an absolute URL, whatever wire it is asked for.
func TestRegistry_MultiEndpointModelsAllBuildAURL(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	wires := []string{registry.DefaultFormat, "claude", registry.FormatOpenAIResponses}
	checked := 0
	for _, entry := range index.All() {
		if len(entry.Transports) == 0 {
			continue
		}
		connector := provider.NewOpenCode(entry)
		for _, model := range entry.Models {
			for _, wire := range wires {
				request := provider.Request{Provider: entry, Model: model, Wire: wire}
				url, err := connector.Endpoint(request, provider.StaticKey("ep", "k", "sk"))
				if err != nil {
					t.Fatalf("Endpoint(%s/%s, wire %s) error = %v", entry.ID, model.ID, wire, err)
				}
				if !strings.HasPrefix(url, "https://") {
					t.Fatalf("Endpoint(%s/%s, wire %s) = %q, want an absolute URL", entry.ID, model.ID, wire, url)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no multi-endpoint model was checked; the registry changed shape")
	}
	t.Logf("checked %d model/wire pairs across the multi-endpoint entries", checked)
}

// TestRegistry_ClaudeOnlyModelsReachTheMessagesEndpoint pins the endpoint choice
// for the ids that declare the claude wire: asked in claude, they answer on the
// messages URL, and they are not sent to the chat endpoint where the body would be
// unreadable.
func TestRegistry_ClaudeOnlyModelsReachTheMessagesEndpoint(t *testing.T) {
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("loading the embedded registry: %v", err)
	}
	entry, ok := index.Provider("opencode-zen")
	if !ok {
		t.Fatal("opencode-zen is missing from the embedded registry")
	}
	connector := provider.NewOpenCode(entry)

	claudeOnly := 0
	for _, model := range entry.Models {
		if !openCodeTestSupports(model, "claude") || openCodeTestSupports(model, registry.DefaultFormat) {
			continue
		}
		claudeOnly++
		url, err := connector.Endpoint(
			provider.Request{Provider: entry, Model: model, Wire: "claude"},
			provider.StaticKey("ep", "k", "sk"))
		if err != nil {
			t.Fatalf("Endpoint(%s) error = %v", model.ID, err)
		}
		if !strings.HasSuffix(url, "/zen/v1/messages") {
			t.Fatalf("Endpoint(%s) = %q, want the messages endpoint", model.ID, url)
		}
	}
	if claudeOnly == 0 {
		t.Fatal("no claude-only model was found; the entry changed shape")
	}
	t.Logf("checked %d claude-only models", claudeOnly)
}

// openCodeTestSupports reports whether a model declares a wire, reading the empty
// list as "every wire", which is the rule the connector applies.
func openCodeTestSupports(model registry.Model, wire string) bool {
	if len(model.SupportedFormats) == 0 {
		return true
	}
	for _, format := range model.SupportedFormats {
		if format == wire {
			return true
		}
	}
	return false
}
