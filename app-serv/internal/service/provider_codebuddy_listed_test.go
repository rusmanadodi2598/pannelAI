// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/provider_codebuddy_listed_test.go
// @for       The shipped registry data serving both CodeBuddy regions to the panel.
// @uses      internal/registry, internal/service, context, testing.
// @reason    Draft 011 F1 asked for two registry entries the operator can see, and the
//
//	criterion it closed on was a test that reads them through the list and
//	detail use cases rather than an assertion about the YAML file. The panel
//	decides from has_oauth and auth_modes whether it offers a connect flow and
//	which credential a form may ask for, so those two fields are pinned here
//	together with the shape the port depends on: the endpoint each region
//	answers, the headers it identifies itself with, the usage endpoint the
//	quota read resolves, and the state round that makes its connect flow
//	servable. A fixture index would prove the filtering; only the embedded
//	index proves the data.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package service

import (
	"context"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

type codebuddyExpectation struct {
	id        string
	alias     string
	baseURL   string
	usageURL  string
	ideType   string
	userAgent string
}

var codebuddyRegions = []codebuddyExpectation{
	{
		id: "codebuddy-cn", alias: "cbcn",
		baseURL:  "https://copilot.tencent.com/v2/chat/completions",
		usageURL: "https://copilot.tencent.com/v2/billing/meter/get-user-resource",
		ideType:  "CLI", userAgent: "CLI/2.108.1 CodeBuddy/2.108.1",
	},
	{
		id: "codebuddy-intl", alias: "cbai",
		baseURL:  "https://www.codebuddy.ai/v2/chat/completions",
		usageURL: "https://www.codebuddy.ai/v2/billing/meter/get-user-resource",
		ideType:  "IDE", userAgent: "IDE/2.108.1 CodeBuddy/2.108.1",
	},
}

// embeddedProviderIndex loads the registry the binary actually ships with, so a
// claim about a provider is a claim about the file the gateway reads.
func embeddedProviderIndex(t *testing.T) *registry.Index {
	t.Helper()
	index, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	return index
}

func listedProvider(t *testing.T, svc *ProviderService, id string) registry.Provider {
	t.Helper()
	rows, total, err := svc.List(context.Background(), ProviderFilter{}, 1, 500)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if int64(len(rows)) != total {
		t.Fatalf("List() answered %d of %d: the page window is too small to prove an entry is served", len(rows), total)
	}
	for _, row := range rows {
		if row.Entry.ID == id {
			return row.Entry
		}
	}
	t.Fatalf("List() did not serve %q among %d entries", id, len(rows))
	return registry.Provider{}
}

// TestProviderService_ListServesBothCodeBuddyRegions pins draft 011 F1: the two
// entries the reference split out of the old combined `codebuddy` are served
// unfiltered, each keeping its own domain, alias, headers, usage endpoint and
// connect flow.
func TestProviderService_ListServesBothCodeBuddyRegions(t *testing.T) {
	svc, err := NewProviderService(ProviderServiceDeps{Index: embeddedProviderIndex(t)})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}

	for _, want := range codebuddyRegions {
		t.Run(want.id, func(t *testing.T) {
			entry := listedProvider(t, svc, want.id)

			if entry.Hidden {
				t.Fatalf("%q is hidden: the list filters it out, so the panel cannot show it", want.id)
			}
			if entry.Alias != want.alias || entry.UIAlias != want.alias {
				t.Fatalf("%q aliases = %q/%q, want %q on both: the operator addresses the model by alias",
					want.id, entry.Alias, entry.UIAlias, want.alias)
			}
			if entry.Category != "oauth" || !entry.HasOAuth {
				t.Fatalf("%q category/has_oauth = %q/%v, want oauth/true: the panel renders the connect section from has_oauth",
					want.id, entry.Category, entry.HasOAuth)
			}
			if !containsString(entry.AuthModes, "oauth") || !containsString(entry.AuthModes, "apikey") {
				t.Fatalf("%q auth_modes = %v, want both oauth and apikey", want.id, entry.AuthModes)
			}
			if entry.Transport.BaseURL != want.baseURL {
				t.Fatalf("%q base_url = %q, want %q", want.id, entry.Transport.BaseURL, want.baseURL)
			}
			if !entry.Transport.ForceStream {
				t.Fatalf("%q declares no force_stream: the connector's declaration has nothing to agree with", want.id)
			}
			if entry.Transport.Usage.URL != want.usageURL {
				t.Fatalf("%q usage.url = %q, want %q: the quota read resolves its endpoint here", want.id, entry.Transport.Usage.URL, want.usageURL)
			}
			if got := entry.Transport.Headers["X-IDE-Type"]; got != want.ideType {
				t.Fatalf("%q X-IDE-Type = %q, want %q", want.id, got, want.ideType)
			}
			if got := entry.Transport.Headers["User-Agent"]; got != want.userAgent {
				t.Fatalf("%q User-Agent = %q, want %q", want.id, got, want.userAgent)
			}
			if got := entry.Transport.Headers["x-codebuddy-request"]; got != "1" {
				t.Fatalf("%q x-codebuddy-request = %q, want 1: the endpoint refuses the request without it", want.id, got)
			}
			if !entry.OAuth.StateExchangeFlow() {
				t.Fatalf("%q declares no state round: its connect flow would fall back to a connector we do not need", want.id)
			}

			detail, err := svc.Detail(context.Background(), want.id)
			if err != nil {
				t.Fatalf("Detail(%q) error = %v", want.id, err)
			}
			if detail.Entry.ID != want.id {
				t.Fatalf("Detail() answered %q, want %q", detail.Entry.ID, want.id)
			}
		})
	}
}

// TestProviderService_ListSearchFindsOnlyTheCodeBuddyRegions pins the panel's
// search box: typing the provider's name reaches both regions and nothing else,
// so an operator is never offered a third combined CodeBuddy.
func TestProviderService_ListSearchFindsOnlyTheCodeBuddyRegions(t *testing.T) {
	svc, err := NewProviderService(ProviderServiceDeps{Index: embeddedProviderIndex(t)})
	if err != nil {
		t.Fatalf("NewProviderService() error = %v", err)
	}

	rows, total, err := svc.List(context.Background(), ProviderFilter{Q: "codebuddy"}, 1, 50)
	if err != nil {
		t.Fatalf("List(q=codebuddy) error = %v", err)
	}
	if total != int64(len(codebuddyRegions)) || len(rows) != len(codebuddyRegions) {
		t.Fatalf("List(q=codebuddy) answered %d of %d, want %d", len(rows), total, len(codebuddyRegions))
	}
	for _, want := range codebuddyRegions {
		found := false
		for _, row := range rows {
			found = found || row.Entry.ID == want.id
		}
		if !found {
			t.Fatalf("List(q=codebuddy) omitted %q", want.id)
		}
	}
}
