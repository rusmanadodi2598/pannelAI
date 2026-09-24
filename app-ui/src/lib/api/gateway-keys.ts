// Gateway key calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.3.

import {
	schemaCreateGatewayKeyForm,
	schemaCreatedGatewayKey,
	schemaGatewayKey,
	schemaGatewayKeyList,
	schemaUpdateGatewayKeyForm,
	type CreateGatewayKeyForm,
	type CreatedGatewayKey,
	type GatewayKey,
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

// Rename, disable, and enable all answer with the updated key row and never with a plaintext key, so the
// update parses through the list row's schema rather than the create response's. Parsing the create shape
// here made every rename and toggle fail with a drift error that the row then discarded.
export function updateGatewayKey(
	id: string,
	form: UpdateGatewayKeyForm
): Promise<ApiResult<GatewayKey>> {
	return apiRequest<UpdateGatewayKeyForm, GatewayKey>({
		method: 'PATCH',
		path: `/gateway-keys/${encodeURIComponent(id)}`,
		schema: schemaGatewayKey,
		body: form,
		bodySchema: schemaUpdateGatewayKeyForm
	});
}

// The terminal action of SPEC-API §7.3. The domain and the spec call it revocation; the reference
// labels the control Delete (`EndpointPageClient.js:647-668`) and the screen follows the reference,
// so the panel's button reads Delete while this function keeps the domain's word.
export function revokeGatewayKey(id: string): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: `/gateway-keys/${encodeURIComponent(id)}`,
		schema: emptyResponse
	});
}
