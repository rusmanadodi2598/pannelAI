// Provider registry calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.4.
//
// The registry is a data set the API owns, so every filter this module sends is one the API accepts, and
// the list is always read with the server's paging. Nothing here fetches the whole registry for the panel
// to filter, which is what §6.3's pagination discipline rules out.
//
// The OAuth routes are here rather than in a module of their own because their paths are the provider's:
// §7.4 nests them under `/providers/{provider_id}`, and the section that calls them is the provider detail
// screen. The two device routes belong beside the other four for the same reason, even though the panel
// drives those by asking repeatedly instead of once.

import {
	schemaModelTestResult,
	schemaModelTestSweep,
	type ModelTestResult,
	type ModelTestSweep
} from '$lib/schemas/model-test';
import {
	schemaOAuthDevicePoll,
	schemaOAuthDevicePollBody,
	schemaOAuthDeviceStart,
	schemaOAuthRefresh,
	schemaOAuthRefreshBody,
	schemaOAuthStart,
	schemaOAuthStatus,
	type OAuthDevicePoll,
	type OAuthDevicePollBody,
	type OAuthDeviceStart,
	type OAuthRefresh,
	type OAuthRefreshBody,
	type OAuthStart,
	type OAuthStatus
} from '$lib/schemas/oauth';
import {
	schemaProviderDetail,
	schemaProviderList,
	schemaProviderModelList,
	type ProviderDetail,
	type ProviderList,
	type ProviderModelList,
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

// The models the provider's own upstream answers, which is the list a custom node's screen imports from
// (§7.4, the reference's "Import from /models"). The route reads no query: it asks the provider itself,
// so there is nothing for the panel to filter on.
//
// A node with no reachable connection has nothing to ask, so the answer is an empty list rather than an
// error, and the screen renders that answer instead of predicting it.
export function listProviderModels(providerId: string): Promise<ApiResult<ProviderModelList>> {
	return apiRequest<void, ProviderModelList>({
		method: 'GET',
		path: `/providers/${encodeURIComponent(providerId)}/models`,
		schema: schemaProviderModelList
	});
}

// Probes one model of one provider through the data plane (§7.4, draft 017 §4.10). A model that refused is
// an answered row, not a failed request, so only a refusal of the request itself reaches `error`.
export function testProviderModel(
	providerId: string,
	modelId: string
): Promise<ApiResult<ModelTestResult>> {
	return apiRequest<{ model_id: string }, ModelTestResult>({
		method: 'POST',
		path: `/providers/${encodeURIComponent(providerId)}/models/test`,
		schema: schemaModelTestResult,
		body: { model_id: modelId }
	});
}

// Probes a bounded sweep of the provider's chat models. The limit is sent explicitly rather than left to
// the server's default, so the button's own words and the request cannot disagree about what one click
// spends.
export function testProviderModels(
	providerId: string,
	limit: number
): Promise<ApiResult<ModelTestSweep>> {
	return apiRequest<{ limit: number }, ModelTestSweep>({
		method: 'POST',
		path: `/providers/${encodeURIComponent(providerId)}/test-models`,
		schema: schemaModelTestSweep,
		body: { limit }
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

// Starts a device round. The gateway mints the PKCE pair, the nonce and the machine id and keeps them; the
// answer is the link the operator opens plus the code every later poll comes back with.
export function startProviderOAuthDevice(providerId: string): Promise<ApiResult<OAuthDeviceStart>> {
	return apiRequest<void, OAuthDeviceStart>({
		method: 'POST',
		path: `/providers/${encodeURIComponent(providerId)}/oauth/device/start`,
		schema: schemaOAuthDeviceStart
	});
}

// Asks about one device round once. The gateway performs exactly one upstream attempt per call, so how
// often the vendor is asked is this screen's schedule, not a queue the gateway holds.
export function pollProviderOAuthDevice(
	providerId: string,
	deviceCode: string
): Promise<ApiResult<OAuthDevicePoll>> {
	return apiRequest<OAuthDevicePollBody, OAuthDevicePoll>({
		method: 'POST',
		path: `/providers/${encodeURIComponent(providerId)}/oauth/device/poll`,
		schema: schemaOAuthDevicePoll,
		body: { device_code: deviceCode },
		bodySchema: schemaOAuthDevicePollBody
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
