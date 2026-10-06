// Tests that the quota page skips providers that publish no quota.
//
// @file      internal/service/quota_published_index_test.go
// @for       Pins that an account behind a provider with no usage endpoint is left out of the published answer.
// @uses      context, internal/domain, internal/registry, internal/service, testing, time.
// @reason    "Not polled yet" promises a poll that will never arrive, and a provider that publishes no quota at all is not waiting for anything. Telling those two apart is the difference between a queue and a capability, so the filter that decides it gets its own test.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-02
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// pageQuotaIndex answers the one question the account filter asks: does this provider
// publish a quota at all. The lane that does not must be left out of the answer rather
// than told "not polled yet", which promises a poll that will never happen.
type pageQuotaIndex struct{ publishing map[string]bool }

func (i pageQuotaIndex) Provider(name string) (registry.Provider, bool) {
	if !i.publishing[name] {
		return registry.Provider{ID: name}, false
	}
	return registry.Provider{ID: name, Features: registry.Features{Usage: true}}, true
}

func (i pageQuotaIndex) All() []registry.Provider {
	out := make([]registry.Provider, 0, len(i.publishing))
	for name := range i.publishing {
		provider, _ := i.Provider(name)
		out = append(out, provider)
	}
	return out
}

func (i pageQuotaIndex) Categories() []string { return nil }

func TestPagePublishedSkipsProvidersThatPublishNoQuota(t *testing.T) {
	cache := &countingPublishedCache{byID: map[string]domain.PublishedQuota{}}
	svc := &QuotaService{
		publishedCache: cache, clock: time.Now,
		providers: pageQuotaIndex{publishing: map[string]bool{"glm": true}},
	}

	accounts := []domain.QuotaAccount{
		{EndpointID: "ep_glm", ProviderID: "glm"},
		{EndpointID: "ep_custom", ProviderID: "openai-compatible-XYZ"},
		{EndpointID: "ep_virtual", ProviderID: ""},
	}
	answers, err := svc.PagePublished(context.Background(), accounts)
	if err != nil {
		t.Fatalf("PagePublished() error = %v", err)
	}
	if len(answers) != 1 || answers[0].EndpointID != "ep_glm" {
		t.Fatalf("PagePublished() = %+v, want only the account whose provider publishes a quota", answers)
	}
	if !answers[0].NeverPolled {
		t.Fatal("a capable provider with no cached answer yet is exactly the never-polled case")
	}
}
