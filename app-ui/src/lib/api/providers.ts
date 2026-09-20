// Provider registry calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.4.
//
// The registry is a data set the API owns, so every filter this module sends is one the API accepts, and
// the list is always read with the server's paging. Nothing here fetches the whole registry for the panel
// to filter, which is what §6.3's pagination discipline rules out.
//
// The three OAuth routes are here rather than in a module of their own because their paths are the
// provider's: §7.4 nests them under `/providers/{provider_id}`, and the section that calls them is the
// provider detail screen.

import {
	schemaOAuthRefresh,
	schemaOAuthRefreshBody,
	schemaOAuthStart,
	schemaOAuthStatus,
	type OAuthRefresh,
	type OAuthRefreshBody,
	type OAuthStart,
	type OAuthStatus
} from '$lib/schemas/oauth';
import {
	schemaProviderDetail,
	schemaProviderList,
	type ProviderDetail,
	type ProviderList,
	type ProviderQuery
} from '$lib/schemas/provider';
import { apiRequest, type ApiResult } from './client';

export type ListQuery = {
	page?: number;
	per_page?: number;
};

export type ProviderListQuery = ListQuery & ProviderQuery;

export function listProviders(query: ProviderListQuery = {}): Promise<ApiResult<ProviderList>> {
	return apiRequest<void, ProviderList>({
		method: 'GET',
		path: '/providers',
		schema: schemaProviderList,
		query
	});
}

export function getProvider(id: string): Promise<ApiResult<ProviderDetail>> {
	return apiRequest<void, ProviderDetail>({
		method: 'GET',
		path: `/providers/${encodeURIComponent(id)}`,
		schema: schemaProviderDetail
	});
}

// Starts an authorization and answers the URL the operator is sent to. The body is omitted: the gateway
// derives its own callback from its configured base URL, and §6.3 sends the browser there rather than to a
// redirect the panel picked.
export function startProviderOAuth(providerId: string): Promise<ApiResult<OAuthStart>> {
	return apiRequest<void, OAuthStart>({
		method: 'POST',
		path: `/providers/${encodeURIComponent(providerId)}/oauth/start`,
		schema: schemaOAuthStart
	});
}

// The provider's connected accounts and the flow the gateway would offer. Not paginated: the route reads
// one provider's OAuth endpoints, and the panel renders every row it answers.
export function providerOAuthStatus(providerId: string): Promise<ApiResult<OAuthStatus>> {
	return apiRequest<void, OAuthStatus>({
		method: 'GET',
		path: `/providers/${encodeURIComponent(providerId)}/oauth/status`,
		schema: schemaOAuthStatus
	});
}

// Refreshes one account, or every account that is due when no id is named. The API fails fast on the first
// refusal, so the answer is either the accounts that moved or the concrete reason none did.
export function refreshProviderOAuth(
	providerId: string,
	endpointId?: string
): Promise<ApiResult<OAuthRefresh>> {
	return apiRequest<OAuthRefreshBody, OAuthRefresh>({
		method: 'POST',
		path: `/providers/${encodeURIComponent(providerId)}/oauth/refresh`,
		schema: schemaOAuthRefresh,
		body: endpointId === undefined ? {} : { endpoint_id: endpointId },
		bodySchema: schemaOAuthRefreshBody
	});
}
