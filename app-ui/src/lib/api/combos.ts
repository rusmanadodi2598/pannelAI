// Combo calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.7.
//
// Create and patch carry the same body, so they share one schema and one type: §7.7 makes a patch a full
// replace, and a panel that sent a delta would silently clear the fields it left out. The test route is
// absent because §7.7 places it in P2.

import { schemaCombo, schemaComboList, type Combo, type ComboList } from '$lib/schemas/combo';
import type { ComboBody } from '$lib/schemas/combo-form';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

export type ListQuery = {
	page?: number;
	per_page?: number;
};

export function listCombos(query: ListQuery = {}): Promise<ApiResult<ComboList>> {
	return apiRequest<void, ComboList>({
		method: 'GET',
		path: '/combos',
		schema: schemaComboList,
		query
	});
}

export function getCombo(id: string): Promise<ApiResult<Combo>> {
	return apiRequest<void, Combo>({
		method: 'GET',
		path: `/combos/${encodeURIComponent(id)}`,
		schema: schemaCombo
	});
}

export function createCombo(body: ComboBody): Promise<ApiResult<Combo>> {
	return apiRequest<ComboBody, Combo>({
		method: 'POST',
		path: '/combos',
		schema: schemaCombo,
		body
	});
}

// A full replace, not a delta: the API validates name, strategy, and models as required on this route.
export function updateCombo(id: string, body: ComboBody): Promise<ApiResult<Combo>> {
	return apiRequest<ComboBody, Combo>({
		method: 'PATCH',
		path: `/combos/${encodeURIComponent(id)}`,
		schema: schemaCombo,
		body
	});
}

// The API answers CONFLICT when an alias still targets this combo's name (§7.7), which the caller renders
// as the reason rather than as a generic failure.
export function deleteCombo(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/combos/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}
