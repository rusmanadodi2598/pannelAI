// Gateway key calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.3.

import {
	schemaCreateGatewayKeyForm,
	schemaCreatedGatewayKey,
	schemaGatewayKeyList,
	schemaUpdateGatewayKeyForm,
	type CreateGatewayKeyForm,
	type CreatedGatewayKey,
	type GatewayKeyList,
	type UpdateGatewayKeyForm
} from '$lib/schemas/gateway-key';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

export type ListQuery = {
	page?: number;
	per_page?: number;
};

export function listGatewayKeys(query: ListQuery = {}): Promise<ApiResult<GatewayKeyList>> {
	return apiRequest<void, GatewayKeyList>({
		method: 'GET',
		path: '/gateway-keys',
		schema: schemaGatewayKeyList,
		query
	});
}

export function createGatewayKey(
	form: CreateGatewayKeyForm
): Promise<ApiResult<CreatedGatewayKey>> {
	return apiRequest<CreateGatewayKeyForm, CreatedGatewayKey>({
		method: 'POST',
		path: '/gateway-keys',
		schema: schemaCreatedGatewayKey,
		body: form,
		bodySchema: schemaCreateGatewayKeyForm
	});
}

// Rename, disable, and enable all answer with the updated key, so one schema covers them.
export function updateGatewayKey(
	id: string,
	form: UpdateGatewayKeyForm
): Promise<ApiResult<CreatedGatewayKey>> {
	return apiRequest<UpdateGatewayKeyForm, CreatedGatewayKey>({
		method: 'PATCH',
		path: `/gateway-keys/${encodeURIComponent(id)}`,
		schema: schemaCreatedGatewayKey,
		body: form,
		bodySchema: schemaUpdateGatewayKeyForm
	});
}

export function revokeGatewayKey(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/gateway-keys/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}
