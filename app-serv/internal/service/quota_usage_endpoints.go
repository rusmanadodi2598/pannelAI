// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/quota_usage_endpoints.go
// @for       Maps one provider's declared usage block into the shape the quota fetcher reads.
// @uses      internal/registry, internal/service/quotafetch.
// @reason    The registry spells a provider's usage hosts under several keys and each family
//
//	asks one of them, so forwarding only `url` left a family that asks
//	`quota_url`, `quota_api_url`, `urls`, `oauth_url` or `user_url` calling
//	an empty address. Mapping the whole block once, here, is what keeps that
//	a single fix rather than fourteen patches as each family lands.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-02
package service

import (
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service/quotafetch"
)

// publishedAccountData hands a family the few account facts its usage endpoint names,
// which are not secrets and are already on the aggregate: the Google cloudcode families
// meter a project, and Grok addresses a billable user by id and email. Without them a
// family must discover the value by asking for it, which costs an extra provider call on
// every poll — the reason the credential builder fills them rather than leaving the map
// empty and letting each fetcher bootstrap.
//
// nil when nothing is present, so an account with no project never sends an empty
// "projectId" that a provider could read as a real, blank one.
func publishedAccountData(credential *domain.OAuthCredential, account domain.EndpointAccount) map[string]string {
	data := map[string]string{}

	if credential != nil {
		if project := strings.TrimSpace(credential.ProjectID); project != "" {
			data["projectId"] = project
		}
		if email := strings.TrimSpace(credential.AccountEmail); email != "" {
			data["email"] = email
		}
		if user := strings.TrimSpace(credential.AccountID); user != "" {
			data["userId"] = user
		}
	}
	if _, hasEmail := data["email"]; !hasEmail {
		if email := strings.TrimSpace(account.Email); email != "" {
			data["email"] = email
		}
	}

	if len(data) == 0 {
		return nil
	}
	return data
}

// publishedCredentials is the shape both auth lanes hand the fetcher: the registry's usage
// endpoints, the provider's own headers, and the account facts. Only the opened secret differs
// between them, so the caller sets that one field — and the facts travel with both, because a
// family that has to discover its project or user pays a second provider call for it on every poll.
func publishedCredentials(entry registry.Provider, account domain.EndpointAccount, credential *domain.OAuthCredential) quotafetch.Credentials {
	return quotafetch.Credentials{
		Endpoints:            usageEndpoints(entry.Transport.Usage),
		UsageHeaders:         entry.Transport.Headers,
		ProviderSpecificData: publishedAccountData(credential, account),
	}
}

// usageEndpoints copies the registry's usage block into the fetcher's own mirror. The
// copy is explicit field by field, not a struct conversion, so a key the registry adds
// shows up here as a compile-time gap in one place instead of as a family silently
// asking the wrong host at runtime.
func usageEndpoints(declared registry.UsageConfig) quotafetch.UsageEndpoints {
	return quotafetch.UsageEndpoints{
		URL:                 declared.URL,
		URLs:                declared.URLs,
		TokenURL:            declared.TokenURL,
		QuotaURL:            declared.QuotaURL,
		QuotaAPIURL:         declared.QuotaAPIURL,
		LoadCodeAssistURL:   declared.LoadCodeAssistURL,
		LoadProjectAPIURL:   declared.LoadProjectAPIURL,
		OrgURL:              declared.OrgURL,
		SettingsURL:         declared.SettingsURL,
		LimitsPath:          declared.LimitsPath,
		ResetCreditsConsume: declared.ResetCreditsConsumeURL,
		ResetCredits:        declared.ResetCreditsURL,
		QuotaSummaryAPIURL:  declared.QuotaSummaryAPIURL,
		UserURL:             declared.UserURL,
		OAuthURL:            declared.OAuthURL,
		CWHost:              declared.CWHost,
		QHost:               declared.QHost,
	}
}
