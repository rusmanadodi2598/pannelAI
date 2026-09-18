// Provider registry calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.4.
//
// The registry is a data set the API owns, so every filter this module sends is one the API accepts, and
// the list is always read with the server's paging. Nothing here fetches the whole registry for the panel
// to filter, which is what §6.3's pagination discipline rules out.

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
