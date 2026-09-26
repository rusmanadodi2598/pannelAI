// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_embedded_test.go
// @for       The connector gap report and URL resolution driven against the
//
//	REAL embedded registry, not a fixture.
//
// @uses      testing, strings, internal/registry.
// @reason    A lookup asserted only against synthetic providers proves the
//
//	lookup, not the data it will meet. A provider that is listed but
//	unanswerable is a routing failure waiting for traffic, so both the
//	report a maintainer greps and the URL every entry resolves are
//	measured over the document the binary embeds.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package provider

import (
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// TestConnectors_UnsupportedSurfacesTheGap covers the report a maintainer greps:
// a provider that needs custom handling but has none is a routing failure
// waiting for traffic.
func TestConnectors_UnsupportedSurfacesTheGap(t *testing.T) {
	idx, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	connectors, err := NewConnectors(DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	unsupported := connectors.Unsupported(idx)
	// The report must be sorted and free of duplicates, so it diffs cleanly
	// between runs.
	seen := make(map[string]struct{}, len(unsupported))
	for i, id := range unsupported {
		if _, dup := seen[id]; dup {
			t.Fatalf("Unsupported() repeats %q", id)
		}
		seen[id] = struct{}{}
		if i > 0 && unsupported[i-1] > id {
			t.Fatalf("Unsupported() is not sorted at %q", id)
		}
	}

	// A plain key provider on a natively supported format must NOT be reported:
	// it is served by the fallback, which is the point of the fallback.
	if _, reported := seen["deepseek"]; reported {
		t.Fatal("deepseek is served by the fallback and must not be reported as unsupported")
	}
	// A provider on a bespoke wire format must be reported.
	if _, reported := seen["commandcode"]; !reported {
		t.Fatal("commandcode speaks a bespoke format and must be reported as unsupported without a connector")
	}
}

// TestDefault_EmbeddedProvidersAllResolve drives the real registry: every
// provider must produce a connector and a usable URL, which is what makes the
// embedded registry self-sufficient.
func TestDefault_EmbeddedProvidersAllResolve(t *testing.T) {
	idx, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	connectors, err := NewConnectors(DefaultFactory)
	if err != nil {
		t.Fatalf("NewConnectors() error = %v", err)
	}

	withURL := 0
	for _, entry := range idx.All() {
		connector := connectors.For(entry)
		if connector.ProviderID() != entry.ID {
			t.Fatalf("connector for %s reports id %s", entry.ID, connector.ProviderID())
		}
		url, err := connector.Endpoint(Request{Provider: entry}, Credential{APIKey: "sk-x"})
		if entry.Transport.BaseURL == "" && len(entry.Transport.BaseURLs) == 0 {
			// A media-only provider legitimately carries no chat URL.
			if err == nil {
				t.Fatalf("provider %s declares no URL but produced %q", entry.ID, url)
			}
			continue
		}
		if err != nil {
			t.Fatalf("provider %s: Endpoint() error = %v", entry.ID, err)
		}
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			t.Fatalf("provider %s produced a non-absolute URL %q", entry.ID, url)
		}
		withURL++
	}
	if withURL < 30 {
		t.Fatalf("only %d providers produced a URL, want the curated chat-capable set", withURL)
	}
}
